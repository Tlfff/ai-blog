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
)

// GrpcCmd represents the http command
var GrpcCmd = &cobra.Command{
	Use:   "grpc",
	Short: "leo-grpc",
	Long:  `blog-grpc 提供 API 接口`,
	Run: func(cmd *cobra.Command, args []string) {
		l, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
		logger := slog.New(slog.LevelAdapt(l))
		app, cancel, err := wireGrpcApp()
		if err != nil {
			logger.Fatal(err)
		}
		defer cancel()

		// 初始化app
		leoApp := leo.NewApp(
			leo.Logger(logger),
			leo.Runners(app),
		)
		// 运行leoApp
		if err := leoApp.Run(context.Background()); err != nil {
			logger.Info(err.Error())
			return
		}
	},
}

func newGrpcApp(
	cfg *conf.Config,
	gs *server.GrpcServer,
) *lgrpc.Server {
	grpcServer := lgrpc.NewServer(
		int(cfg.GetServer().Grpc.Port),
		lgrpc.Name("hello"),
		lgrpc.ID("12345"),
	)
	gs.Register(grpcServer)
	actuatorServer := actuator.New(
		16060,
		actuator.Handlers(grpcServer.ActuatorHandler()),
		actuator.HealthCheckers(grpcServer.HealthChecker()),
	)
	_ = actuatorServer

	return grpcServer
}
