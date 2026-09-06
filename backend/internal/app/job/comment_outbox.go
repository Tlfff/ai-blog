package job

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

const (
	commentOutboxBatchSize    = 100                    // commentOutboxBatchSize 是单批最大消息数。
	commentOutboxPollInterval = 500 * time.Millisecond // commentOutboxPollInterval 是空闲轮询间隔。
	commentOutboxRetryBase    = time.Second            // commentOutboxRetryBase 是失败重试基础间隔。
	commentOutboxRetryMax     = time.Minute            // commentOutboxRetryMax 是失败重试最大间隔。
)

// CommentOutboxRelay 将评论事务 Outbox 至少一次发布到 Kafka。
type CommentOutboxRelay struct {
	outbox    comment.OutboxRepository // outbox 提供待发布消息和发布状态持久化。
	publisher comment.EventPublisher   // publisher 提供同步 Kafka 发布确认。
	now       func() time.Time         // now 提供可测试的当前时间。
}

// NewCommentOutboxRelay 创建评论 Outbox 发布任务。
func NewCommentOutboxRelay(outbox comment.OutboxRepository, publisher comment.EventPublisher) *CommentOutboxRelay {
	// 1. 启动阶段拒绝缺少 Outbox 或消息发布能力
	if outbox == nil || publisher == nil {
		panic("评论 Outbox 发布任务缺少必要依赖")
	}
	return &CommentOutboxRelay{outbox: outbox, publisher: publisher, now: time.Now}
}

// PublishBatch 发布一批到期 Outbox 消息。
func (r *CommentOutboxRelay) PublishBatch(ctx context.Context) error {
	// 1. 查询当前到期且尚未完成的消息
	now := r.now()
	messages, err := r.outbox.ListPending(ctx, commentOutboxBatchSize, now)
	if err != nil {
		return fmt.Errorf("查询评论 Outbox: %w", err)
	}

	// 2. 逐条同步确认发布结果，失败消息保留并安排指数退避补发
	var publishErr error
	for _, message := range messages {
		if err := r.publisher.Publish(ctx, message.Event); err != nil {
			nextAttempt := now.Add(commentOutboxRetryDelay(message.Attempts))
			if markErr := r.outbox.MarkFailed(ctx, message.Event.EventID, err.Error(), nextAttempt); markErr != nil {
				publishErr = errors.Join(publishErr, fmt.Errorf("发布评论事件 %s: %w", message.Event.EventID, err), fmt.Errorf("记录评论事件失败: %w", markErr))
				continue
			}
			publishErr = errors.Join(publishErr, fmt.Errorf("发布评论事件 %s: %w", message.Event.EventID, err))
			continue
		}
		if err := r.outbox.MarkPublished(ctx, message.Event.EventID, now); err != nil {
			publishErr = errors.Join(publishErr, fmt.Errorf("确认评论事件 %s 已发布: %w", message.Event.EventID, err))
		}
	}
	return publishErr
}

// Run 持续补发评论 Outbox，直到 Leo 生命周期结束。
func (r *CommentOutboxRelay) Run(ctx context.Context) error {
	// 1. 启动后立即发布一批，单次依赖失败记录日志但不终止补偿任务
	if err := r.PublishBatch(ctx); err != nil && ctx.Err() == nil {
		log.L().WithContext(ctx).Error("发布评论 Outbox 失败", err)
	}
	ticker := time.NewTicker(commentOutboxPollInterval)
	defer ticker.Stop()

	// 2. 按固定间隔扫描到期消息，退出由进程上下文统一控制
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.PublishBatch(ctx); err != nil && ctx.Err() == nil {
				log.L().WithContext(ctx).Error("发布评论 Outbox 失败", err)
			}
		}
	}
}

// commentOutboxRetryDelay 计算有上限的指数退避间隔。
func commentOutboxRetryDelay(attempts int) time.Duration {
	// 1. 防止异常次数导致移位溢出
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 6 {
		return commentOutboxRetryMax
	}
	delay := commentOutboxRetryBase * time.Duration(1<<attempts)
	if delay > commentOutboxRetryMax {
		return commentOutboxRetryMax
	}
	return delay
}
