package ipregion

import (
	"os"
	"path/filepath"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// TestNewConfiguredResolverRequiresExplicitPaths 验证资源配置缺失时启动失败。
func TestNewConfiguredResolverRequiresExplicitPaths(t *testing.T) {
	if _, _, err := NewConfiguredResolver(&conf.Config{}); err == nil {
		t.Fatal("NewConfiguredResolver() error = nil, want missing path error")
	}
}

// TestNewConfiguredResolverUsesAbsolutePaths 验证配置解析不依赖当前工作目录并可安全关闭。
func TestNewConfiguredResolverUsesAbsolutePaths(t *testing.T) {
	v4Path, v6Path := resourcePaths(t)
	config := &conf.Config{Ipv4XdbPath: v4Path, Ipv6XdbPath: v6Path}
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	}()

	resolver, cleanup, err := NewConfiguredResolver(config)
	if err != nil {
		t.Fatalf("NewConfiguredResolver() error = %v", err)
	}
	if got := resolver.Resolve("223.5.5.5"); got != "浙江" {
		t.Fatalf("Resolve() = %q, want 浙江", got)
	}
	cleanup()
	cleanup()
}

// TestNewConfiguredResolverRejectsRelativeAndMismatchedPaths 验证路径和 XDB 版本错误导致启动失败。
func TestNewConfiguredResolverRejectsRelativeAndMismatchedPaths(t *testing.T) {
	v4Path, v6Path := resourcePaths(t)
	tests := []struct {
		name   string // name 是测试场景名称。
		v4Path string // v4Path 是 IPv4 XDB 配置路径。
		v6Path string // v6Path 是 IPv6 XDB 配置路径。
	}{
		{name: "相对路径", v4Path: "assets/ipregion/ip2region.xdb", v6Path: "assets/ipregion/ip2region_v6.xdb"},
		{name: "资源不存在", v4Path: filepath.Join(t.TempDir(), "missing.xdb"), v6Path: v6Path},
		{name: "版本不匹配", v4Path: v6Path, v6Path: v4Path},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &conf.Config{Ipv4XdbPath: test.v4Path, Ipv6XdbPath: test.v6Path}
			if _, _, err := NewConfiguredResolver(config); err == nil {
				t.Fatal("NewConfiguredResolver() error = nil")
			}
		})
	}
}
