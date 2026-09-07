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

启动器逐行发布同一 Binlog 包中的全部 ROW，避免 Meilisync 0.1.3 只处理 `rows[0]`；每批 Meilisearch 任务成功后才提交 Redis Binlog 进度；同一 MySQL ROW 包全部行处理完成后才推进包尾位点，worker Key 使用带所有权的单实例租约；refresh Key 同样使用排他租约拒绝并发全量刷新。索引设置固定搜索字段优先级，并将 `status` 声明为可过滤字段。后端查询仍会强制追加 `status = 3`，Meilisync 不负责隐藏草稿或软删除文档。

## 暂停、恢复与全量刷新

```bash
# 暂停会等待活跃增量进程刷完已接收事件并确认；没有活跃进程时直接记录暂停状态
python3 -m meilisync_plugin.runner -c configs/meilisync.yml pause

# 使用 articles__refresh__<uuid> 临时索引、正式索引设置和同一 ArticlePlugin 全量导入
python3 -m meilisync_plugin.runner -c configs/meilisync.yml refresh -t articles

# 核对 MySQL 与稳定 articles 索引数量，并清理非活动的遗留临时索引
python3 -m meilisync_plugin.runner -c configs/meilisync.yml check -t articles

# 手工暂停时显式恢复；进程按 Redis 保存的 Binlog 文件与位置重连
python3 -m meilisync_plugin.runner -c configs/meilisync.yml resume
```

`refresh` 一次仅刷新一张配置表，并会先取得排他 refresh 租约，再自动请求暂停。命令记录候选 Binlog 进度，在同一 REPEATABLE READ 一致性快照中按主键游标分批导入并等待任务完成；数量和设置通过后使用 Index Swap 原子交换，在旧索引删除前提交候选进度。可确认的交换前、校验或进度提交失败会保持或原子恢复稳定索引；若 Redis 响应与回读都无法确认，则保留新旧索引并停止自动清理，避免错误回滚造成事件跳过。旧索引删除失败时同样保留，由后续 `check` 按规则重试清理。若刷新前已经由运维手工暂停，命令会保留暂停状态，由运维执行 `resume`。Redis 使用 `<progress.key>:control` 保存暂停令牌，`:worker` 保存增量进程租约，`:refresh-lock` 保存刷新租约，`:refresh` 保存带 owner 的活动临时索引；`refresh`、`check` 和手工 `resume` 通过同一 refresh 租约串行执行，防止刷新期间提前恢复或误删临时索引；MySQL 仍只读取原 `articles` 表。

## 验证

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s meilisync_plugin -p "test_*.py"
MEILISYNC_INTEGRATION=1 PYTHONDONTWRITEBYTECODE=1 python3 -m unittest meilisync_plugin.test_integration
```

集成测试需要 Docker，使用临时 MySQL、Redis 和 Meilisearch 容器验证 INSERT、UPDATE、DELETE ROW 事件、暂停/恢复、Redis 进度续传、全量刷新、Index Swap、一致性检查和临时索引清理，并在结束后自动清理。
