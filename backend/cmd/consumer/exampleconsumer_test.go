package consumer

import (
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// TestConsumerActuatorPortUsesConfigAndFallback 验证 Consumer 管理端口配置。
func TestConsumerActuatorPortUsesConfigAndFallback(t *testing.T) {
	// 1. 显式配置优先，缺省配置使用独立默认端口
	if got := consumerActuatorPort(&conf.Config{Management: &conf.Management{Port: 19001}}); got != 19001 {
		t.Fatalf("configured port = %d", got)
	}
	if got := consumerActuatorPort(&conf.Config{}); got != 16061 {
		t.Fatalf("fallback port = %d", got)
	}
}
