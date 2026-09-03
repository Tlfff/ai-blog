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
	//
	job.ProviderJobSet,
	// 基础层
	clients.ProviderClientsSet,
	// 领域层
	domain.DomainProviderAppSet,
)

func NewBlogJob() (*job.BlogJob, func(), error) {
	panic(wire.Build(
		ProviderSet,
	))
}
