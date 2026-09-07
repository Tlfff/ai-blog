//go:build wireinject
// +build wireinject

package job

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/job"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	conf.ProviderSet,
	job.ProviderJobSet,
	clients.ProviderClientsSet,
	domain.DomainProviderAppSet,
	newJobApplication,
)

// NewBlogJob 组装由 Leo 生命周期管理的后台任务应用。
func NewBlogJob() (*jobApplication, func(), error) {
	// 1. 由 Wire 生成实际依赖组装实现
	panic(wire.Build(
		ProviderSet,
	))
}
