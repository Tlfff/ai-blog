# `/configs`

Configuration file templates or default configs.

Put your `confd` or `consul-template` template files here.

## 用户认证与本地 IP 地区配置

本地配置或 Nacos 配置需要提供以下字段；XDB 路径必须是绝对路径，避免依赖进程工作目录：

```yaml
trusted_proxy_cidrs:
  - "10.0.0.0/8"
ipv4_xdb_path: "/assets/ipregion/ip2region.xdb"
ipv6_xdb_path: "/assets/ipregion/ip2region_v6.xdb"
```

Docker 镜像会将仓库中的 `assets/` 复制到 `/assets/`。非容器环境应配置实际部署目录下的绝对路径。

## 正文图片 MinIO 配置

正文图片使用 MinIO 预签名地址直传。`endpoint` 只填写主机和端口，不包含 URL scheme；密钥示例必须替换为部署环境的安全配置：

```yaml
data:
  object_storage:
    endpoint: "minio.example.test:9000"
    access_key: "replace-with-access-key"
    secret_key: "replace-with-secret-key"
    bucket: "article-images"
    use_ssl: true
    public_url: "https://cdn.example.test/article-images"
    image_extensions:
      - "jpg"
      - "jpeg"
      - "png"
      - "gif"
      - "webp"
```

## 文章浏览 Kafka 配置

HTTP 进程发布浏览事件，Consumer 进程处理失败后写入死信主题：

```yaml
data:
  kafka:
    producer:
      article_view:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "article-view"
      article_view_dead_letter:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "article-view-dlq"
    consumer:
      article_view:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "article-view"
        group_id: "article-view-projector"
        message_buffer_size: 16
```


## 评论事件 Outbox 与文章评论数投影 Kafka 配置

Consumer 进程持续补发评论事务 Outbox，并由独立消费组维护文章 `comment_count`；处理重试耗尽后写入死信主题：

```yaml
data:
  kafka:
    producer:
      comment_event:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "comment-event"
      comment_event_dead_letter:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "comment-event-dlq"
    consumer:
      comment_event:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "comment-event"
        group_id: "article-comment-count-projector"
        message_buffer_size: 16
```


## 文章点赞 Outbox、计数投影与缓存重建配置

Consumer 进程补发文章点赞事务 Outbox，文章上下文消费独立 Topic 维护 `like_count`；HTTP 进程从 MySQL 点赞事实周期重建 Redis Set：

```yaml
data:
  kafka:
    producer:
      like_event:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "article-like-event"
      like_event_dead_letter:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "article-like-event-dlq"
    consumer:
      like_event:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "article-like-event"
        group_id: "article-like-count-projector"
        message_buffer_size: 16
```

## 开放 gRPC 认证配置

开放 gRPC 的内部调用使用 HS256 JWT，外部合作方使用 HMAC-SHA256。所有密钥都必须由部署系统注入，示例占位值不得用于真实环境：

```yaml
server:
  grpc:
    auth:
      jwt_issuer: "blog-internal"
      jwt_secret: "replace-with-at-least-32-byte-jwt-secret"
      jwt_clock_skew_seconds: 5
      hmac_time_window_seconds: 60
      nonce_ttl_seconds: 60
      hmac_access_keys:
        partner-a: "replace-with-at-least-32-byte-hmac-secret"
```

内部 JWT 必须包含 `iss`、`sub`、`iat`、`exp`，且签名算法固定为 HS256。外部请求携带 `x-access-key-id`、`x-signature`、`x-timestamp`、`x-nonce`；签名规范串按以下顺序用换行符连接，并对整个字符串计算 HMAC-SHA256 后使用十六进制编码：

```text
/full.grpc.Service/Method
<access-key-id>
<unix-timestamp-seconds>
<nonce>
<sha256-of-deterministic-protobuf-request>
```

`x-timestamp` 不能来自未来，且与服务端当前 Unix 秒之差必须严格小于 `hmac_time_window_seconds`；Nonce 长度为 16～128 字节，并在同一 Access Key 下只能成功使用一次。认证失败响应和日志不得包含 JWT、密钥、签名或 Nonce。

## 评论点赞事件与投影重建

评论点赞事件复用 `like_event` Topic 和同一消费组，由消费者按 `article.*`、`comment.*` 事件类型路由到各自上下文；当前不会发布评论点赞通知。评论点赞 Redis 集合和 `comments.like_count` 会由 MySQL 点赞事实周期重建。

## 通知 MongoDB 与文章点赞消费组

通知上下文使用独立 MongoDB 数据库；文章点赞通知订阅与点赞计数订阅复用同一 Topic，但必须使用独立消费组：

```yaml
data:
  mongo:
    uri: "mongodb://notification-user:replace-with-password@mongo.example.test:27017"
    database: "blog_notification"
    connect_timeout: "5s"
  kafka:
    consumer:
      article_like_notification:
        bootstrap_servers: "kafka.example.test:9092"
        topic: "article-like-event"
        group_id: "article-like-notification"
        message_buffer_size: 16
```

## Meilisearch 与 Meilisync 配置

HTTP 搜索通过 `data.meilisearch.endpoint` 和 `data.meilisearch.api_key` 连接固定的 `articles` 索引。独立 Meilisync 进程读取 `configs/meilisync.yml`，使用 MySQL ROW Binlog 同步 `blog.articles`，并在 Redis 保存同步进度。索引设置、增量同步暂停/恢复、全量刷新和一致性检查步骤见 `meilisync_plugin/README.md`。控制状态使用进度 Key 的 `:control` 与 `:worker` 派生 Key，不写入 MySQL 搜索字段或同步表。
