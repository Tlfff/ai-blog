package service

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"fmt"
	"github.com/google/wire"
	"net"
)

// ServiceProviderAppSet is service providers.
var ServiceProviderAppSet = wire.NewSet(
	NewBlogServer,
	NewBookServer,
	NewUserServer,
	NewArticleServer,
	NewCommentServer,
	NewLikeServer,
	ProvideTrustedProxyCIDRs,
)

// ServiceGrpcProviderAppSet is service providers.
var ServiceGrpcProviderAppSet = wire.NewSet(
	NewGrpcBlogServer,
	NewGrpcBookServer,
)

// ProvideTrustedProxyCIDRs 提供受信代理网段，限制转发头只能由可信代理声明。
func ProvideTrustedProxyCIDRs(config *conf.Config) ([]string, error) {
	// 1. 未配置代理时只信任直接连接地址
	if config == nil {
		return []string{}, nil
	}

	// 2. 启动时拒绝无效 CIDR，避免运行时静默忽略安全配置
	cidrs := append([]string(nil), config.GetTrustedProxyCidrs()...)
	for _, cidr := range cidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return nil, fmt.Errorf("解析受信代理 CIDR %q: %w", cidr, err)
		}
	}
	return cidrs, nil
}
