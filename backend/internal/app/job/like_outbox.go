package job

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

const (
	likeOutboxBatchSize    = 100                    // likeOutboxBatchSize 是单批最大消息数。
	likeOutboxPollInterval = 500 * time.Millisecond // likeOutboxPollInterval 是空闲轮询间隔。
	likeOutboxRetryBase    = time.Second            // likeOutboxRetryBase 是失败重试基础间隔。
	likeOutboxRetryMax     = time.Minute            // likeOutboxRetryMax 是失败重试最大间隔。
)

// LikeOutboxRelay 将点赞事务 Outbox 至少一次发布到 Kafka。
type LikeOutboxRelay struct {
	outbox    like.OutboxRepository // outbox 提供待发布消息和发布状态持久化。
	publisher like.EventPublisher   // publisher 提供同步 Kafka 发布确认。
	now       func() time.Time      // now 提供可测试的当前时间。
}

// NewLikeOutboxRelay 创建点赞 Outbox 发布任务。
func NewLikeOutboxRelay(outbox like.OutboxRepository, publisher like.EventPublisher) *LikeOutboxRelay {
	// 1. 启动阶段拒绝缺少 Outbox 或消息发布能力
	if outbox == nil || publisher == nil {
		panic("点赞 Outbox 发布任务缺少必要依赖")
	}
	return &LikeOutboxRelay{outbox: outbox, publisher: publisher, now: time.Now}
}

// PublishBatch 发布一批到期 Outbox 消息。
func (r *LikeOutboxRelay) PublishBatch(ctx context.Context) error {
	// 1. 查询当前到期且尚未完成的消息
	now := r.now()
	messages, err := r.outbox.ListPending(ctx, likeOutboxBatchSize, now)
	if err != nil {
		return fmt.Errorf("查询点赞 Outbox: %w", err)
	}

	// 2. 逐条同步确认发布结果，失败消息保留并安排指数退避补发
	var publishErr error
	for _, message := range messages {
		if err := r.publisher.Publish(ctx, message.Event); err != nil {
			nextAttempt := now.Add(likeOutboxRetryDelay(message.Attempts))
			if markErr := r.outbox.MarkFailed(ctx, message.Event.EventID, err.Error(), nextAttempt); markErr != nil {
				publishErr = errors.Join(publishErr, fmt.Errorf("发布点赞事件 %s: %w", message.Event.EventID, err), fmt.Errorf("记录点赞事件失败: %w", markErr))
				continue
			}
			publishErr = errors.Join(publishErr, fmt.Errorf("发布点赞事件 %s: %w", message.Event.EventID, err))
			continue
		}
		if err := r.outbox.MarkPublished(ctx, message.Event.EventID, now); err != nil {
			publishErr = errors.Join(publishErr, fmt.Errorf("确认点赞事件 %s 已发布: %w", message.Event.EventID, err))
		}
	}
	return publishErr
}

// Run 持续补发点赞 Outbox，直到 Leo 生命周期结束。
func (r *LikeOutboxRelay) Run(ctx context.Context) error {
	// 1. 启动后立即发布一批，单次依赖失败记录日志但不终止补偿任务
	if err := r.PublishBatch(ctx); err != nil && ctx.Err() == nil {
		log.L().WithContext(ctx).Error("发布点赞 Outbox 失败", err)
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
				log.L().WithContext(ctx).Error("发布点赞 Outbox 失败", err)
			}
		}
	}
}

// likeOutboxRetryDelay 计算有上限的指数退避间隔。
func likeOutboxRetryDelay(attempts int) time.Duration {
	// 1. 防止异常次数导致移位溢出
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 6 {
		return likeOutboxRetryMax
	}
	delay := likeOutboxRetryBase * time.Duration(1<<attempts)
	if delay > likeOutboxRetryMax {
		return likeOutboxRetryMax
	}
	return delay
}
