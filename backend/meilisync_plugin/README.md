# 文章搜索同步

## 前置条件

- MySQL 开启 Binary Log，并设置 `binlog_format=ROW`；同步账号具备读取 `blog.articles` 和 Binlog 的权限。
- Redis 用于保存 `blog:meilisync:articles:binlog-position`，Meilisearch 使用固定 `articles` 索引。
- 在 `backend/` 中安装依赖：`pip install -r meilisync_plugin/requirements.txt`。
- 部署环境通过配置管理覆盖 `configs/meilisync.yml` 中的连接地址和凭据，禁止将生产密钥提交到仓库。

## 初始化与增量同步

```bash
cd backend
MEILISEARCH_URL=http://127.0.0.1:7700 MEILISEARCH_API_KEY= python3 -m meilisync_plugin.index_settings
python3 -m meilisync_plugin.runner -c configs/meilisync.yml start
```

兼容启动器会在读取 Redis 进度后重建惰性连接，并逐行发布同一 Binlog 包中的全部 ROW，避免 Meilisync 0.1.3 跨事件循环复用连接或只处理 `rows[0]`；索引设置固定搜索字段优先级，并将 `status` 声明为可过滤字段。后端查询仍会强制追加 `status = 3`，Meilisync 不负责隐藏草稿或软删除文档。

全量刷新、Index Swap 与一致性检查不属于本工单，由 #15 继续实现。

## 验证

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest meilisync_plugin.test_articles meilisync_plugin.test_runner
MEILISYNC_INTEGRATION=1 PYTHONDONTWRITEBYTECODE=1 python3 -m unittest meilisync_plugin.test_integration
```

集成测试需要 Docker，使用临时 MySQL、Redis 和 Meilisearch 容器验证应用实际单行 INSERT、UPDATE、DELETE ROW 事件，并在结束后自动清理。
