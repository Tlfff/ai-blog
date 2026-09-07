package clients

import (
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// TestNewMongoClientRejectsMissingAndInvalidConfiguration 验证启动阶段拒绝不安全的通知存储配置。
func TestNewMongoClientRejectsMissingAndInvalidConfiguration(t *testing.T) {
	// 1. 缺失 MongoDB 配置时不得启动通知存储
	if _, _, err := NewMongoClient(&conf.Config{}); err == nil {
		t.Fatal("missing mongo config was accepted")
	}

	// 2. 非法连接超时在建立网络连接前失败
	config := &conf.Config{Data: &conf.Data{Mongo: &conf.Mongo{Uri: "mongodb://example.test", Database: "notification", ConnectTimeout: "invalid"}}}
	if _, _, err := NewMongoClient(config); err == nil {
		t.Fatal("invalid mongo timeout was accepted")
	}
}
