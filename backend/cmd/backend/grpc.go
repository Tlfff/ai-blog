package server

import (
	"context"
	"os"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/server"
	"codeup.aliyun.com/qimao/leo/leo"
	"codeup.aliyun.com/qimao/leo/leo/actuator"
	"codeup.aliyun.com/qimao/leo/leo/log"
	"codeup.aliyun.com/qimao/leo/leo/log/slog"
	"codeup.aliyun.com/qimao/leo/leo/transport/lgrpc"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// GrpcCmd 启动开放 gRPC 服务。
var GrpcCmd = &cobra.Command{
	Use:   "grpc",
	Short: "leo-grpc",
	Long:  `blog-grpc 提供 API 接口`,
	Run: func(cmd *cobra.Command, args []string) {
		// 1. 初始化统一日志与 Wire 依赖
		l, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
		logger := slog.New(slog.LevelAdapt(l))
		app, cancel, err := wireGrpcApp()
		if err != nil {
			logger.Fatal(err)
		}
		defer cancel()

		// 2. 将 gRPC 与 Actuator 交给 Leo Runner 管理
		leoApp := leo.NewApp(
			leo.Logger(logger),
			leo.Runners(app),
		)
		// 3. 阻塞运行并沿用 Leo 的统一退出处理
		if err := leoApp.Run(context.Background()); err != nil {
			logger.Info(err.Error())
			return
		}
	},
}

// grpcApplication 聚合 gRPC 传输服务与 Actuator，并统一交给 Leo 管理生命周期。
type grpcApplication struct {
	server   *lgrpc.Server    // server 是注册业务接口和认证拦截器的 gRPC Server。
	actuator *actuator.Server // actuator 是暴露健康检查与传输信息的管理 Server。
}

// Run 并发运行 gRPC 与 Actuator，并在上下文取消时沿用各自的优雅退出流程。
func (app *grpcApplication) Run(ctx context.Context) error {
	// 1. 让两个 Runner 共享取消信号并分别执行优雅退出
	return leo.MutilRunner(app.server, app.actuator).Run(ctx)
}

// newGrpcApp 创建带统一认证、Actuator 和 Leo 生命周期的 gRPC 应用。
func newGrpcApp(
	cfg *conf.Config,
	gs *server.GrpcServer,
	authInterceptor grpc.UnaryServerInterceptor,
) *grpcApplication {
	// 1. 创建并注册带认证拦截器的 Leo gRPC Server
	grpcServer := lgrpc.NewServer(
		int(cfg.GetServer().Grpc.Port),
		lgrpc.Name("hello"),
		lgrpc.ID("12345"),
		lgrpc.UnaryInterceptors(authInterceptor),
	)
	gs.Register(grpcServer)

	// 2. 保留 gRPC 健康检查和传输信息管理端点
	actuatorServer := actuator.New(
		16060,
		actuator.Handlers(grpcServer.ActuatorHandler()),
		actuator.HealthCheckers(grpcServer.HealthChecker()),
	)

	// 3. 聚合为单个 Leo Runner，避免绕开统一退出流程
	return &grpcApplication{server: grpcServer, actuator: actuatorServer}
}
