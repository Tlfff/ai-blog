package job

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

const articleLikeCacheRebuildInterval = 10 * time.Minute

// ArticleLikeCacheRebuildJob 周期使用 MySQL 事实重建 Redis 点赞集合。
type ArticleLikeCacheRebuildJob struct {
	rebuilder like.CacheRebuilder // rebuilder 提供点赞事实到 Redis 的重建能力。
	interval  time.Duration       // interval 是两次完整重建间隔。
}

// NewArticleLikeCacheRebuildJob 创建文章点赞缓存重建任务。
func NewArticleLikeCacheRebuildJob(rebuilder like.CacheRebuilder) *ArticleLikeCacheRebuildJob {
	// 1. 启动阶段拒绝缺少点赞缓存重建能力
	if rebuilder == nil {
		panic("文章点赞缓存重建任务缺少必要依赖")
	}
	return &ArticleLikeCacheRebuildJob{rebuilder: rebuilder, interval: articleLikeCacheRebuildInterval}
}

// Run 启动时和固定周期从 MySQL 重建 Redis 点赞集合。
func (j *ArticleLikeCacheRebuildJob) Run(ctx context.Context) error {
	// 1. 启动重建失败只记录日志，HTTP 与点赞事实写入仍可继续
	if err := j.rebuilder.RebuildArticleLikeCache(ctx); err != nil && ctx.Err() == nil {
		log.L().WithContext(ctx).Error("重建文章点赞缓存失败", err)
	}
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// 2. 周期故障等待下一轮补偿，退出由 Leo 生命周期统一控制
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := j.rebuilder.RebuildArticleLikeCache(ctx); err != nil && ctx.Err() == nil {
				log.L().WithContext(ctx).Error("重建文章点赞缓存失败", err)
			}
		}
	}
}
