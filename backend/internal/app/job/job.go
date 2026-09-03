package job

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

// BlogJob 表示博客后台任务应用。
type BlogJob struct {
	svc *book.HelloworldService
}

// NewJob 创建博客后台任务应用。
func NewJob(svc *book.HelloworldService) *BlogJob {
	return &BlogJob{
		svc: svc,
	}
}

// Run 执行博客后台任务。
func (job *BlogJob) Run() {
	// ==============  指定内容脚本 ======================
	err := job.svc.Job()
	if err != nil {
		log.Error(err)
	}

}
