package job

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
)

// task 定义脚本任务执行能力。
type task interface {
	// Job 执行一次任务主体。
	Job() error
}

// BlogJob 表示由 Leo 生命周期管理的博客后台任务。
type BlogJob struct {
	task task // task 是当前任务主体。
}

// NewJob 创建博客后台任务。
func NewJob(service *book.HelloworldService) *BlogJob {
	// 1. Wire 入口将存量任务服务适配为可测试任务接口
	if service == nil {
		panic("博客后台任务缺少任务服务")
	}
	return newBlogJob(service)
}

// newBlogJob 创建可注入测试替身的后台任务。
func newBlogJob(current task) *BlogJob {
	// 1. 启动阶段拒绝缺少任务主体
	if current == nil {
		panic("博客后台任务缺少任务主体")
	}
	return &BlogJob{task: current}
}

// Run 执行一次任务并等待 Leo 统一退出信号。
func (job *BlogJob) Run(ctx context.Context) error {
	// 1. 任务失败立即返回，由 Leo 取消同进程其他 Runner
	if err := job.task.Job(); err != nil {
		return err
	}

	// 2. 任务完成后保留 Actuator，收到退出信号时优雅结束
	<-ctx.Done()
	return nil
}
