package server

import (
	"context"
	"os"
	"time"

	appjob "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/job"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/eventstream"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/middleware"
	"codeup.aliyun.com/qimao/leo/leo"
	"codeup.aliyun.com/qimao/leo/leo/actuator"
	"codeup.aliyun.com/qimao/leo/leo/actuator/health"
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

// httpApplication 聚合 HTTP Server 和正文图片删除恢复 Runner。
type httpApplication struct {
	server             *ginhttp.Server                   // server 是博客 HTTP 传输服务。
	reconciler         *appjob.ArticleDeletionReconciler // reconciler 是正文图片删除恢复任务。
	userSessionCleanup *appjob.UserSessionCleanupJob     // userSessionCleanup 是用户会话收敛补偿任务。
	hotRank            *appjob.ArticleHotRankJob         // hotRank 是文章热榜启动及整点重建任务。
	viewEvents         *eventstream.ArticleViewPublisher // viewEvents 是文章浏览事件异步发送 Runner。
}

// Run 通过 Leo 生命周期并发运行 HTTP 服务和恢复任务。
func (app *httpApplication) Run(ctx context.Context) error {
	// 1. 复用 Leo 多 Runner 编排和统一退出机制
	return leo.MutilRunner(app.server, app.reconciler, app.userSessionCleanup, app.hotRank, app.viewEvents).Run(ctx)
}

// ActuatorHandler 返回 HTTP Server 的管理端点处理器。
func (app *httpApplication) ActuatorHandler() actuator.Handler {
	// 1. 管理端点继续由原 HTTP Server 提供
	return app.server.ActuatorHandler()
}

// HealthChecker 返回 HTTP Server 的健康检查器。
func (app *httpApplication) HealthChecker() health.Checker {
	// 1. 健康状态继续由原 HTTP Server 提供
	return app.server.HealthChecker()
}

// newApp 创建包含传输服务、对象恢复、热榜和浏览事件任务的 HTTP 应用。
//
// 参数说明：
//   - cfg：HTTP Server 和中间件配置。
//   - httpServer：生成路由的聚合注册器。
//   - reconciler：正文图片删除恢复任务。
//   - userSessionCleanup：用户会话收敛补偿任务。
//   - hotRank：文章热榜启动及整点重建任务。
//   - viewEvents：文章浏览事件异步发送任务。
func newApp(cfg *conf.Config, httpServer ginhttp.RegisterServer, reconciler *appjob.ArticleDeletionReconciler, userSessionCleanup *appjob.UserSessionCleanupJob, hotRank *appjob.ArticleHotRankJob, viewEvents *eventstream.ArticleViewPublisher) *httpApplication {
	// 1. 初始化 HTTP 中间件和生成路由
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

	// 2. 解析超时并创建 Leo HTTP Server
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

	// 3. 聚合全部由 Leo 生命周期管理的 Runner
	return &httpApplication{server: httpServers, reconciler: reconciler, userSessionCleanup: userSessionCleanup, hotRank: hotRank, viewEvents: viewEvents}
}
