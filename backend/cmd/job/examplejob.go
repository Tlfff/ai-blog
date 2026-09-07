package job

import (
	"context"
	"fmt"
	"os"
	"time"

	appjob "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/job"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/leo/leo"
	"codeup.aliyun.com/qimao/leo/leo/actuator"
	"codeup.aliyun.com/qimao/leo/leo/log"
	"codeup.aliyun.com/qimao/leo/leo/log/slog"
	"github.com/spf13/cobra"
)

// blogJobCmd 启动由 Leo 管理的博客后台任务。
var blogJobCmd = &cobra.Command{
	Use:   "blog",
	Short: "blog-job",
	Long:  `运行博客任务`,
	Run: func(cmd *cobra.Command, args []string) {
		// 1. 初始化统一日志与 Wire 依赖
		level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
		if err != nil {
			panic(err)
		}
		log.L().SetLevel(level)
		logger := slog.New(slog.LevelAdapt(level))
		application, cleanup, err := NewBlogJob()
		if err != nil {
			logger.Fatal(err)
		}
		defer cleanup()

		// 2. 任务、Actuator 和退出信号统一交给 Leo 管理
		app := leo.NewApp(leo.Logger(logger), leo.Runners(application))
		if err := app.Run(context.Background()); err != nil {
			logger.Info(err.Error())
		}
	},
}

// jobApplication 聚合后台任务与 Actuator 生命周期。
type jobApplication struct {
	job      *appjob.BlogJob  // job 是一次任务主体和退出等待 Runner。
	actuator *actuator.Server // actuator 是 Job 管理与诊断服务。
}

// Run 并发运行后台任务和 Actuator，并共享优雅退出信号。
func (app *jobApplication) Run(ctx context.Context) error {
	// 1. 任一 Runner 失败时由 Leo 取消另一 Runner
	if err := leo.MutilRunner(app.job, app.actuator).Run(ctx); err != nil {
		return fmt.Errorf("运行博客任务: %w", err)
	}
	return nil
}

// newJobApplication 创建 Job 进程应用。
func newJobApplication(config *conf.Config, current *appjob.BlogJob) *jobApplication {
	// 1. 使用配置管理端口，未配置时保留 Job 独立默认端口
	management := actuator.New(jobActuatorPort(config), actuator.Logger(log.L()), actuator.ShutdownTimeout(10*time.Second))
	return &jobApplication{job: current, actuator: management}
}

// jobActuatorPort 返回 Job 管理端口。
func jobActuatorPort(config *conf.Config) int {
	// 1. 优先使用统一配置，未配置时使用 Job 独立默认端口
	if port := int(config.GetManagement().GetPort()); port > 0 {
		return port
	}
	return 16062
}

// init 注册博客任务子命令。
func init() {
	// 1. 将博客任务加入根 Job 命令
	JobCmd.AddCommand(blogJobCmd)
}
