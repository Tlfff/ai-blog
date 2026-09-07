# Issue #18 真实基础设施验收

业务事件链路使用 MySQL 8、Redis 7、Apache Kafka 4 和 MongoDB 7：

```bash
cd backend
./scripts/integration/issue18.sh
```

搜索恢复链路复用 Meilisync 的真实 MySQL ROW Binlog、Redis 与 Meilisearch 验收：

```bash
cd backend
MEILISYNC_INTEGRATION=1 PYTHONDONTWRITEBYTECODE=1 python3 -m unittest meilisync_plugin.test_integration
```

两套测试均使用独立容器和临时端口，退出时自动清理；业务测试覆盖 Outbox 暂时失败与补发、消费重试、重复消息、Kafka 死信、跨上下文对账、文章互动投影修复、通知幂等、浏览历史和热榜重建。搜索测试覆盖 ROW Binlog INSERT/UPDATE/DELETE、Redis 位点恢复、暂停/恢复、Index Swap、数量检查和临时索引清理。
