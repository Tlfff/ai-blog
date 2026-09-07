#!/usr/bin/env bash
set -euo pipefail

backend_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
suffix="${USER:-user}-$$"
mysql_container="ai-blog-issue18-mysql-${suffix}"
redis_container="ai-blog-issue18-redis-${suffix}"
mongo_container="ai-blog-issue18-mongo-${suffix}"
kafka_container="ai-blog-issue18-kafka-${suffix}"

free_port() {
  python3 - <<'PY'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
}

mysql_port="$(free_port)"
redis_port="$(free_port)"
mongo_port="$(free_port)"
kafka_port="$(free_port)"

cleanup() {
  docker rm -f "$mysql_container" "$redis_container" "$mongo_container" "$kafka_container" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --name "$mysql_container" \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=blog \
  -p "127.0.0.1:${mysql_port}:3306" mysql:8.0 >/dev/null

docker run -d --name "$redis_container" \
  -p "127.0.0.1:${redis_port}:6379" redis:7-alpine >/dev/null

docker run -d --name "$mongo_container" \
  -p "127.0.0.1:${mongo_port}:27017" mongo:7 >/dev/null

docker run -d --name "$kafka_container" \
  -p "127.0.0.1:${kafka_port}:9094" \
  -e KAFKA_NODE_ID=1 \
  -e KAFKA_PROCESS_ROLES=broker,controller \
  -e KAFKA_LISTENERS=INTERNAL://:9092,CONTROLLER://:9093,EXTERNAL://:9094 \
  -e KAFKA_ADVERTISED_LISTENERS="INTERNAL://localhost:9092,EXTERNAL://127.0.0.1:${kafka_port}" \
  -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER \
  -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT \
  -e KAFKA_CONTROLLER_QUORUM_VOTERS=1@localhost:9093 \
  -e KAFKA_INTER_BROKER_LISTENER_NAME=INTERNAL \
  -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 \
  apache/kafka:4.0.0 >/dev/null

wait_until() {
  local name="$1"
  shift
  for _ in $(seq 1 90); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for ${name}" >&2
  return 1
}

wait_until mysql docker exec "$mysql_container" mysqladmin ping -h127.0.0.1 -uroot -proot --silent
sleep 3
wait_until mysql-final docker exec "$mysql_container" mysql -h127.0.0.1 -uroot -proot -e "SELECT 1"
wait_until redis docker exec "$redis_container" redis-cli ping
wait_until mongo docker exec "$mongo_container" mongosh --quiet --eval 'db.runCommand({ping:1}).ok'
wait_until kafka docker exec "$kafka_container" /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list

for migration in "$backend_dir"/scripts/mysql/*.sql; do
  docker exec -i "$mysql_container" mysql --default-character-set=utf8mb4 -h127.0.0.1 -uroot -proot < "$migration"
done

for topic in \
  issue18-comment-events issue18-comment-events-dlq \
  issue18-like-events issue18-like-events-dlq \
  issue18-view-events issue18-view-events-dlq; do
  docker exec "$kafka_container" /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server localhost:9092 --create --if-not-exists \
    --topic "$topic" --partitions 1 --replication-factor 1 >/dev/null
done

cd "$backend_dir"
ISSUE18_INTEGRATION=1 \
ISSUE18_MYSQL_DSN="root:root@tcp(127.0.0.1:${mysql_port})/blog?parseTime=true&loc=Local&charset=utf8mb4" \
ISSUE18_REDIS_ADDR="127.0.0.1:${redis_port}" \
ISSUE18_MONGO_URI="mongodb://127.0.0.1:${mongo_port}" \
ISSUE18_KAFKA_BOOTSTRAP="127.0.0.1:${kafka_port}" \
GOCACHE="${GOCACHE:-/tmp/ai-blog-issue18-gocache}" \
GOFLAGS="${GOFLAGS:--mod=readonly}" \
go test -tags=integration -run TestIssue18CrossContextEventChainsAndRecovery -count=1 -v ./internal/integration
