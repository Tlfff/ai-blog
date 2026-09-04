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
