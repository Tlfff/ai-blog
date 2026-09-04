package ipregion

import (
	"fmt"
	"path/filepath"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// NewConfiguredResolver 根据 Leo 配置初始化并校验双栈本地 IP 地区解析器。
func NewConfiguredResolver(config *conf.Config) (*Resolver, func(), error) {
	// 1. 要求配置同时提供不依赖工作目录的绝对双栈资源路径
	if config == nil {
		return nil, nil, fmt.Errorf("缺少 IP 地区库配置")
	}
	if !filepath.IsAbs(config.GetIpv4XdbPath()) || !filepath.IsAbs(config.GetIpv6XdbPath()) {
		return nil, nil, fmt.Errorf("IP 地区库路径必须是绝对路径")
	}

	// 2. 初始化双栈解析器并把关闭函数交给 Wire 生命周期
	resolver, err := NewResolver(config.GetIpv4XdbPath(), config.GetIpv6XdbPath())
	if err != nil {
		return nil, nil, fmt.Errorf("初始化 IP 地区库: %w", err)
	}
	return resolver, resolver.Close, nil
}
