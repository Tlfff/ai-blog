package meilisearch

import (
	"fmt"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/search"
	"github.com/google/wire"
)

// ProviderSet 提供搜索上下文的 Meilisearch 客户端。
var ProviderSet = wire.NewSet(NewConfiguredClient, wire.Bind(new(search.Repository), new(*Client)))

// NewConfiguredClient 校验环境配置并创建 Meilisearch 客户端。
func NewConfiguredClient(config *conf.Config) (*Client, error) {
	// 1. 在组合根初始化期间拒绝缺失的搜索配置
	if config == nil || config.GetData() == nil || config.GetData().GetMeilisearch() == nil {
		return nil, fmt.Errorf("缺少 Meilisearch 配置")
	}

	// 2. 校验连接地址并创建官方 SDK 适配器
	settings := config.GetData().GetMeilisearch()
	return NewClient(settings.GetEndpoint(), settings.GetApiKey())
}
