package job

import (
	"context"
	"time"

	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

const userSessionCleanupInterval = time.Minute

// UserSessionCleanupJob 周期重试密码更新后遗留的会话收敛任务。
type UserSessionCleanupJob struct {
	reconciler userdomain.SessionCleanupReconciler // reconciler 提供持久化补偿任务恢复能力。
	interval   time.Duration                       // interval 是两次补偿扫描的间隔。
}

// NewUserSessionCleanupJob 创建用户会话收敛补偿 Runner。
func NewUserSessionCleanupJob(reconciler userdomain.SessionCleanupReconciler) *UserSessionCleanupJob {
	// 1. 启动阶段拒绝缺少会话补偿能力的任务
	if reconciler == nil {
		panic("用户会话收敛任务缺少必要依赖")
	}
	return &UserSessionCleanupJob{reconciler: reconciler, interval: userSessionCleanupInterval}
}

// Run 按固定周期重试待处理的会话收敛任务。
func (j *UserSessionCleanupJob) Run(ctx context.Context) error {
	// 1. 由 Leo Runner 管理定时器生命周期和进程退出
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// 2. 单轮失败保留任务，等待下一周期继续补偿
			if err := j.reconciler.ReconcileSessionCleanup(ctx); err != nil {
				log.L().WithContext(ctx).Error("恢复用户会话收敛失败", err)
			}
		}
	}
}
