package job

import (
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// TestJobActuatorPortUsesConfigAndFallback 验证 Job 管理端口配置。
func TestJobActuatorPortUsesConfigAndFallback(t *testing.T) {
	// 1. 显式配置优先，缺省配置使用独立默认端口
	if got := jobActuatorPort(&conf.Config{Management: &conf.Management{Port: 19002}}); got != 19002 {
		t.Fatalf("configured port = %d", got)
	}
	if got := jobActuatorPort(&conf.Config{}); got != 16062 {
		t.Fatalf("fallback port = %d", got)
	}
}
