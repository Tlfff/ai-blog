package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

const (
	likeCountConsumeAttempts = 3                      // likeCountConsumeAttempts 是点赞计数消费最大尝试次数。
	likeCountRetryDelay      = 100 * time.Millisecond // likeCountRetryDelay 是重试初始间隔。
)

// LikeCountConsumer 使用 Leo Stream 维护文章点赞数投影。
type LikeCountConsumer struct {
	subscriber stream.Subscriber                    // subscriber 是点赞事件订阅器。
	processor  article.LikeCountProcessor           // processor 提供幂等和乱序安全的投影能力。
	deadLetter article.LikeCountDeadLetterPublisher // deadLetter 提供最终失败消息投递能力。
}

// NewLikeCountConsumer 创建文章点赞数消费者。
func NewLikeCountConsumer(subscriber stream.Subscriber, processor article.LikeCountProcessor, deadLetter article.LikeCountDeadLetterPublisher) *LikeCountConsumer {
	// 1. 启动阶段拒绝缺少订阅、投影或死信依赖
	if subscriber == nil || processor == nil || deadLetter == nil {
		panic("文章点赞计数消费者缺少必要依赖")
	}
	return &LikeCountConsumer{subscriber: subscriber, processor: processor, deadLetter: deadLetter}
}

// Subscriber 返回 Leo Stream 使用的点赞事件订阅器。
func (c *LikeCountConsumer) Subscriber() (stream.Subscriber, error) {
	// 1. 返回启动阶段已校验的订阅器
	return c.subscriber, nil
}

// Handle 解析并重试处理文章点赞计数事件。
func (c *LikeCountConsumer) Handle(ctx context.Context, message *stream.Message) error {
	// 1. 损坏消息不进入领域投影，直接发送死信
	var event article.LikeCountEvent
	if err := json.Unmarshal(message.Payload, &event); err != nil {
		if deadLetterErr := c.deadLetter.PublishLikeCountDeadLetter(ctx, message.Payload, err.Error()); deadLetterErr != nil {
			return errors.Join(err, deadLetterErr)
		}
		return nil
	}

	// 2. 投影事务失败时执行有限指数退避重试
	delay := likeCountRetryDelay
	var processErr error
	for attempt := 0; attempt < likeCountConsumeAttempts; attempt++ {
		processErr = c.processor.ApplyLikeCountEvent(ctx, event)
		if processErr == nil {
			return nil
		}
		if attempt == likeCountConsumeAttempts-1 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(processErr, ctx.Err())
		case <-timer.C:
		}
		delay *= 2
	}

	// 3. 重试耗尽后死信成功即确认源消息，死信失败则交给 Leo Nack
	if err := c.deadLetter.PublishLikeCountDeadLetter(ctx, message.Payload, processErr.Error()); err != nil {
		return errors.Join(fmt.Errorf("处理文章点赞计数事件: %w", processErr), fmt.Errorf("投递点赞计数死信: %w", err))
	}
	return nil
}
