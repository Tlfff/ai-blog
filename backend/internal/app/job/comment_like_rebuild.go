package job

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

const commentLikeRebuildInterval = 10 * time.Minute

// CommentLikeRebuildJob 周期从点赞事实重建评论 Redis 集合和 like_count 投影。
type CommentLikeRebuildJob struct {
	cache    like.CommentCacheRebuilder // cache 提供评论点赞事实到 Redis 的重建能力。
	count    comment.LikeCountRebuilder // count 提供评论点赞数和关系投影重建能力。
	interval time.Duration              // interval 是两次完整重建间隔。
}

// NewCommentLikeRebuildJob 创建评论点赞缓存与计数重建任务。
func NewCommentLikeRebuildJob(cache like.CommentCacheRebuilder, count comment.LikeCountRebuilder) *CommentLikeRebuildJob {
	// 1. 启动阶段拒绝缺少任一重建能力
	if cache == nil || count == nil {
		panic("评论点赞重建任务缺少必要依赖")
	}
	return &CommentLikeRebuildJob{cache: cache, count: count, interval: commentLikeRebuildInterval}
}

// Run 启动时和固定周期从 MySQL 点赞事实重建评论点赞投影。
func (j *CommentLikeRebuildJob) Run(ctx context.Context) error {
	// 1. 启动重建失败只记录日志，HTTP 与点赞事实写入仍可继续
	j.rebuild(ctx)
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// 2. 周期故障等待下一轮补偿，退出由 Leo 生命周期统一控制
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			j.rebuild(ctx)
		}
	}
}

// rebuild 依次恢复评论计数关系投影和 Redis 集合。
func (j *CommentLikeRebuildJob) rebuild(ctx context.Context) {
	// 1. 先重建 MySQL 评论计数和关系版本，保证权威读取恢复
	if err := j.count.RebuildCommentLikeCount(ctx); err != nil && ctx.Err() == nil {
		log.L().WithContext(ctx).Error("重建评论点赞计数失败", err)
	}

	// 2. Redis 是可丢失缓存，独立失败不影响 MySQL 重建结果
	if err := j.cache.RebuildCommentLikeCache(ctx); err != nil && ctx.Err() == nil {
		log.L().WithContext(ctx).Error("重建评论点赞缓存失败", err)
	}
}
