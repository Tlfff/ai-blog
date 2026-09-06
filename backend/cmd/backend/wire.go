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
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/middleware"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/server"

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

// wireGrpcApp 组装开放 gRPC、用户查询、统一认证和基础设施依赖。
func wireGrpcApp() (application *grpcApplication, cleanup func(), err error) {
	// 1. 按配置、基础设施、领域、应用、驱动和进程入口的依赖方向完成组装
	panic(wire.Build(
		conf.ProviderSet,
		// 基础设施 Provider
		clients.ProviderClientsSet,
		// 领域 Provider
		domain.DomainProviderAppSet,
		domain.UserQueryProviderSet,
		domain.UserGRPCAuthProviderSet,
		// 应用与认证 Provider
		service.ServiceGrpcProviderAppSet,
		middleware.GRPCProviderSet,
		// 传输注册 Provider
		server.ProviderGrpcServerSet,
		// Leo gRPC 应用入口
		newGrpcApp,
	))
}
