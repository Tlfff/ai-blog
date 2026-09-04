package service

import (
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// TestProvideTrustedProxyCIDRs 验证受信代理配置复制和非法 CIDR 启动失败。
func TestProvideTrustedProxyCIDRs(t *testing.T) {
	t.Parallel()

	valid := &conf.Config{TrustedProxyCidrs: []string{"10.0.0.0/8"}}
	cidrs, err := ProvideTrustedProxyCIDRs(valid)
	if err != nil || len(cidrs) != 1 || cidrs[0] != "10.0.0.0/8" {
		t.Fatalf("ProvideTrustedProxyCIDRs() = %#v, %v", cidrs, err)
	}
	valid.TrustedProxyCidrs[0] = "192.168.0.0/16"
	if cidrs[0] != "10.0.0.0/8" {
		t.Fatal("provider should return an isolated configuration copy")
	}

	invalid := &conf.Config{TrustedProxyCidrs: []string{"invalid"}}
	if _, err := ProvideTrustedProxyCIDRs(invalid); err == nil {
		t.Fatal("ProvideTrustedProxyCIDRs() error = nil")
	}
}
