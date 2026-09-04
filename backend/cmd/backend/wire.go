//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package server

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/job"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/service"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/server"
	"codeup.aliyun.com/qimao/leo/leo/transport/lgrpc"

	"github.com/google/wire"
)

// wireApp init application.
func wireApp() (application *httpApplication, cleanup func(), err error) {
	panic(wire.Build(
		conf.ProviderSet,
		// 基础层
		clients.ProviderClientsSet,
		// 领域层
		domain.DomainProviderAppSet,
		domain.ArticleReadingProviderSet,
		domain.UserProviderSet,
		// 应用层
		service.ServiceProviderAppSet,
		job.ArticleHTTPJobProviderSet,
		// 驱动层
		server.ProviderServerSet,
		// 服务层
		newApp,
	))
}

// wireGrpcApp init application.
func wireGrpcApp() (applications *lgrpc.Server, cleanup func(), err error) {
	// func wireGrpcApp() (applications *lgrpc.Server, ac *actuator.Server, cleanup func(), err error) {
	panic(wire.Build(
		conf.ProviderSet,
		// 基础层
		clients.ProviderClientsSet,
		// 领域层
		domain.DomainProviderAppSet,
		// 应用层
		service.ServiceGrpcProviderAppSet,
		// 驱动层
		server.ProviderGrpcServerSet,
		// 服务层
		newGrpcApp,
	))
}
