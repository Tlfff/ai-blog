package job

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

// CommentLikeOutboxRelay 将评论点赞事务 Outbox 至少一次发布到 Kafka。
type CommentLikeOutboxRelay struct {
	outbox    like.CommentOutboxRepository // outbox 提供评论点赞待发布消息和状态持久化。
	publisher like.EventPublisher          // publisher 复用点赞事件 Kafka 发布能力。
	now       func() time.Time             // now 提供可测试的当前时间。
}

// NewCommentLikeOutboxRelay 创建评论点赞 Outbox 发布任务。
func NewCommentLikeOutboxRelay(outbox like.CommentOutboxRepository, publisher like.EventPublisher) *CommentLikeOutboxRelay {
	// 1. 启动阶段拒绝缺少 Outbox 或消息发布能力
	if outbox == nil || publisher == nil {
		panic("评论点赞 Outbox 发布任务缺少必要依赖")
	}
	return &CommentLikeOutboxRelay{outbox: outbox, publisher: publisher, now: time.Now}
}

// PublishBatch 发布一批到期评论点赞 Outbox 消息。
func (r *CommentLikeOutboxRelay) PublishBatch(ctx context.Context) error {
	// 1. 查询当前到期且尚未完成的评论点赞消息
	now := r.now()
	messages, err := r.outbox.ListPendingCommentLikes(ctx, likeOutboxBatchSize, now)
	if err != nil {
		return fmt.Errorf("查询评论点赞 Outbox: %w", err)
	}

	// 2. 逐条同步确认发布结果，失败消息保留并安排指数退避补发
	var publishErr error
	for _, message := range messages {
		if err := r.publisher.Publish(ctx, message.Event); err != nil {
			nextAttempt := now.Add(likeOutboxRetryDelay(message.Attempts))
			if markErr := r.outbox.MarkCommentLikeFailed(ctx, message.Event.EventID, err.Error(), nextAttempt); markErr != nil {
				publishErr = errors.Join(publishErr, fmt.Errorf("发布评论点赞事件 %s: %w", message.Event.EventID, err), fmt.Errorf("记录评论点赞事件失败: %w", markErr))
				continue
			}
			publishErr = errors.Join(publishErr, fmt.Errorf("发布评论点赞事件 %s: %w", message.Event.EventID, err))
			continue
		}
		if err := r.outbox.MarkCommentLikePublished(ctx, message.Event.EventID, now); err != nil {
			publishErr = errors.Join(publishErr, fmt.Errorf("确认评论点赞事件 %s 已发布: %w", message.Event.EventID, err))
		}
	}
	return publishErr
}

// Run 持续补发评论点赞 Outbox，直到 Leo 生命周期结束。
func (r *CommentLikeOutboxRelay) Run(ctx context.Context) error {
	// 1. 启动后立即发布一批，单次依赖失败记录日志但不终止补偿任务
	if err := r.PublishBatch(ctx); err != nil && ctx.Err() == nil {
		log.L().WithContext(ctx).Error("发布评论点赞 Outbox 失败", err)
	}
	ticker := time.NewTicker(likeOutboxPollInterval)
	defer ticker.Stop()

	// 2. 按固定间隔扫描到期消息，退出由进程上下文统一控制
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.PublishBatch(ctx); err != nil && ctx.Err() == nil {
				log.L().WithContext(ctx).Error("发布评论点赞 Outbox 失败", err)
			}
		}
	}
}
