package job

import (
	"context"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

// ArticleHotRankJob 在启动时和每小时整点重建文章热榜。
type ArticleHotRankJob struct {
	rebuilder article.HotRankRebuilder // rebuilder 提供 MySQL 到 Redis 的权威重建能力。
	now       func() time.Time         // now 提供可测试的当前时间。
}

// NewArticleHotRankJob 创建文章热榜重建任务。
func NewArticleHotRankJob(rebuilder article.HotRankRebuilder) *ArticleHotRankJob {
	// 1. 启动阶段拒绝缺少热榜重建能力
	if rebuilder == nil {
		panic("文章热榜任务缺少必要依赖")
	}
	return &ArticleHotRankJob{rebuilder: rebuilder, now: time.Now}
}

// Run 在启动时重建一次，之后每小时整点重建。
func (j *ArticleHotRankJob) Run(ctx context.Context) error {
	// 1. 启动时立即从 MySQL 权威数据初始化 Redis 热榜
	if err := j.rebuildSafely(ctx); err != nil {
		return err
	}

	// 2. 等待下一个整点并持续按小时重建，单次错误记录后等待下轮
	for {
		now := j.now()
		next := now.Truncate(time.Hour).Add(time.Hour)
		timer := time.NewTimer(next.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
			if err := j.rebuildSafely(ctx); err != nil {
				log.L().WithContext(ctx).Error("重建文章热榜失败", err)
			}
		}
	}
}

// rebuildSafely 捕获热榜重建 Panic 并转换为可观测错误。
func (j *ArticleHotRankJob) rebuildSafely(ctx context.Context) (err error) {
	// 1. 捕获单次任务 Panic，避免破坏后续整点调度
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("文章热榜重建 panic: %v", recovered)
		}
	}()
	return j.rebuilder.RebuildHotRank(ctx)
}
