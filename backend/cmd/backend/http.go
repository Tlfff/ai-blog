package server

import (
	"context"
	"os"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/middleware"
	"codeup.aliyun.com/qimao/leo/leo"
	"codeup.aliyun.com/qimao/leo/leo/actuator"
	"codeup.aliyun.com/qimao/leo/leo/log"
	"codeup.aliyun.com/qimao/leo/leo/log/slog"
	ginlogmdw "codeup.aliyun.com/qimao/leo/leo/middleware/ginhttp/log"
	"codeup.aliyun.com/qimao/leo/leo/middleware/ginhttp/recovery"
	ginrequestid "codeup.aliyun.com/qimao/leo/leo/middleware/ginhttp/requestid"
	"codeup.aliyun.com/qimao/leo/leo/middleware/ginhttp/sentry"
	"codeup.aliyun.com/qimao/leo/leo/transport/ginhttp"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string
)

// HttpCmd represents the http command
var HttpCmd = &cobra.Command{
	Use:   "http",
	Short: "leo-http",
	Long:  `blog-http 提供 API 接口`,
	Run: func(cmd *cobra.Command, args []string) {
		l, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
		log.L().SetLevel(l)
		logger := slog.New(slog.LevelAdapt(l))
		app, cancel, err := wireApp()
		if err != nil {
			logger.Fatal(err)
		}
		defer cancel()
		actuatorServer := actuator.New(
			3000,
			actuator.Handlers(app.ActuatorHandler()),
			actuator.HealthCheckers(app.HealthChecker()),
		)
		// 初始化app
		leoApp := leo.NewApp(
			leo.Logger(logger),
			leo.Runners(app, actuatorServer),
		)

		// 运行leoApp
		if err := leoApp.Run(context.Background()); err != nil {
			logger.Info(err.Error())
			return
		}
	},
}

func newApp(cfg *conf.Config, httpServer ginhttp.RegisterServer) *ginhttp.Server {

	sentry.SentryInit(cfg.GetServer().GetSentry().ToSentryConfig())
	ginEngine := gin.New()
	ginEngine.Use(
		// 将生成代码响应统一转换为博客对外协议
		middleware.UnifiedResponseMiddleware(),
		// 如果需重写返回结构体，请编写自定义rewrite中间件
		//rewrite.Middleware(),
		// 添加sentry中间件，用于捕获panic 默认false
		sentry.Middleware(true),
		// 自定义recover，panic后依旧返回正常结构
		recovery.Middleware(recovery.HandleRecoveryWithErr),
		// 添加请求id中间件，用于联动排查网关日志与服务日志
		ginrequestid.Middleware(ginrequestid.CustomHeaderKey("X-Request-Id")),
		// 添加logger中间件，所有中间件用到的logger都是从gin.Context中获取的
		middleware.CtxMiddleware("X-Request-Id"),
		// 添加请求日志中间件
		ginlogmdw.Middleware(log.FromContext),
	)
	httpServer.Register(ginEngine)

	ts, err := time.ParseDuration(cfg.GetServer().GetHttp().GetTimeout())
	if err != nil {
		log.Error(err.Error())
	}
	httpConf := cfg.GetServer().GetHttp()
	httpServers := ginhttp.NewServer(
		int(httpConf.GetPort()),
		ginEngine,
		ginhttp.Name(httpConf.Network),
		ginhttp.ID(httpConf.Id),
		ginhttp.ReadTimeout(ts),
	)

	return httpServers
}
