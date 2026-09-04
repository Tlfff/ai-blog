package service

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

const articleDeletionReconcileInterval = time.Minute

// ArticleDeletionReconciler 周期恢复进程中断遗留的正文图片暂存删除。
type ArticleDeletionReconciler struct {
	recovery article.DeletionRecovery // recovery 提供基于数据库关系的对象恢复能力。
	interval time.Duration            // interval 是两次持久化恢复扫描的间隔。
}

// NewArticleDeletionReconciler 创建正文图片暂存删除恢复 Runner。
func NewArticleDeletionReconciler(recovery article.DeletionRecovery) *ArticleDeletionReconciler {
	// 1. 启动阶段拒绝缺少领域恢复能力
	if recovery == nil {
		panic("文章对象删除恢复任务缺少必要依赖")
	}
	return newArticleDeletionReconciler(recovery, articleDeletionReconcileInterval)
}

// newArticleDeletionReconciler 创建可配置扫描间隔的恢复 Runner。
func newArticleDeletionReconciler(recovery article.DeletionRecovery, interval time.Duration) *ArticleDeletionReconciler {
	// 1. 保存领域恢复能力和周期配置
	return &ArticleDeletionReconciler{recovery: recovery, interval: interval}
}

// Run 按固定周期恢复成熟的正文图片暂存删除记录。
func (r *ArticleDeletionReconciler) Run(ctx context.Context) error {
	// 1. 由 Leo Runner 管理定时器生命周期和进程退出
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// 2. 单次依赖故障保留持久化隔离记录，等待下一周期重试
			if err := r.recovery.ReconcileStagedDeletions(ctx); err != nil {
				log.L().WithContext(ctx).Error("恢复正文图片暂存删除失败", err)
			}
		}
	}
}
