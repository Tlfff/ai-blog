package meilisearch

import (
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// TestNewConfiguredClientRejectsMissingConfiguration 验证组合根拒绝缺失或无效的 Meilisearch 配置。
func TestNewConfiguredClientRejectsMissingConfiguration(t *testing.T) {
	// 1. 缺少完整配置时必须在服务启动阶段失败
	for _, config := range []*conf.Config{nil, {}, {Data: &conf.Data{}}, {Data: &conf.Data{Meilisearch: &conf.Meilisearch{Endpoint: "invalid"}}}} {
		if client, err := NewConfiguredClient(config); err == nil || client != nil {
			t.Fatalf("config=%#v client=%#v err=%v", config, client, err)
		}
	}
}

// TestNewConfiguredClientBuildsValidatedSDKAdapter 验证合法配置创建搜索适配器。
func TestNewConfiguredClientBuildsValidatedSDKAdapter(t *testing.T) {
	// 1. 合法 HTTP 地址应通过构造期校验
	client, err := NewConfiguredClient(&conf.Config{Data: &conf.Data{Meilisearch: &conf.Meilisearch{Endpoint: "http://127.0.0.1:7700", ApiKey: "secret"}}})
	if err != nil || client == nil {
		t.Fatalf("client=%#v err=%v", client, err)
	}
}
