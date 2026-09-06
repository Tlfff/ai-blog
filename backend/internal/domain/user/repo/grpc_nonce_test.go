package repo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestSessionRepositoryUsesAtomicSetNXForGRPCNonce 验证用户安全仓储通过 Redis 原子防重放、TTL 和凭据摘要 Key。
func TestSessionRepositoryUsesAtomicSetNXForGRPCNonce(t *testing.T) {
	// 1. 使用内存 Redis 启动真实 go-redis 命令链路
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	// 2. 首次占用成功，重复 Nonce 必须原子失败
	store := NewSessionRepository(client)
	const accessKeyID = "partner-sensitive"
	const nonce = "nonce-sensitive-123"
	reserved, err := store.ReserveGRPCNonce(context.Background(), accessKeyID, nonce, time.Minute)
	if err != nil || !reserved {
		t.Fatalf("first Reserve() = %v, %v", reserved, err)
	}
	reserved, err = store.ReserveGRPCNonce(context.Background(), accessKeyID, nonce, time.Minute)
	if err != nil || reserved {
		t.Fatalf("second Reserve() = %v, %v", reserved, err)
	}

	// 3. Redis Key 不包含明文凭据且保留规定 TTL
	keys := server.Keys()
	if len(keys) != 1 {
		t.Fatalf("redis keys = %#v", keys)
	}
	if strings.Contains(keys[0], accessKeyID) || strings.Contains(keys[0], nonce) {
		t.Fatalf("redis key exposed credential: %q", keys[0])
	}
	if ttl := server.TTL(keys[0]); ttl != time.Minute {
		t.Fatalf("redis nonce ttl = %v, want %v", ttl, time.Minute)
	}
}
