package meilisearch

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/search"
	"github.com/google/wire"
)

// ProviderSet 提供搜索上下文的 Meilisearch 客户端。
var ProviderSet = wire.NewSet(NewConfiguredClient, wire.Bind(new(search.Repository), new(*Client)))

// NewConfiguredClient 从环境配置创建 Meilisearch 客户端。
func NewConfiguredClient(config *conf.Config) *Client {
	// 1. 从统一配置源读取 Meilisearch 连接参数
	settings := config.GetData().GetMeilisearch()
	return NewClient(settings.GetEndpoint(), settings.GetApiKey())
}
