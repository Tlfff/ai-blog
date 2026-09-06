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
	commentCountConsumeAttempts = 3                      // commentCountConsumeAttempts 是评论计数消费最大尝试次数。
	commentCountRetryDelay      = 100 * time.Millisecond // commentCountRetryDelay 是重试初始间隔。
)

// CommentCountConsumer 使用 Leo Stream 维护文章评论数投影。
type CommentCountConsumer struct {
	subscriber stream.Subscriber                       // subscriber 是评论事件订阅器。
	processor  article.CommentCountProcessor           // processor 提供幂等和乱序安全的投影能力。
	deadLetter article.CommentCountDeadLetterPublisher // deadLetter 提供最终失败消息投递能力。
}

// NewCommentCountConsumer 创建文章评论数消费者。
func NewCommentCountConsumer(subscriber stream.Subscriber, processor article.CommentCountProcessor, deadLetter article.CommentCountDeadLetterPublisher) *CommentCountConsumer {
	// 1. 启动阶段拒绝缺少订阅、投影或死信依赖
	if subscriber == nil || processor == nil || deadLetter == nil {
		panic("文章评论计数消费者缺少必要依赖")
	}
	return &CommentCountConsumer{subscriber: subscriber, processor: processor, deadLetter: deadLetter}
}

// Subscriber 返回 Leo Stream 使用的评论事件订阅器。
func (c *CommentCountConsumer) Subscriber() (stream.Subscriber, error) {
	// 1. 返回启动阶段已校验的订阅器
	return c.subscriber, nil
}

// Handle 解析并重试处理评论计数事件。
func (c *CommentCountConsumer) Handle(ctx context.Context, message *stream.Message) error {
	// 1. 损坏消息不进入领域投影，直接发送死信
	var event article.CommentCountEvent
	if err := json.Unmarshal(message.Payload, &event); err != nil {
		if deadLetterErr := c.deadLetter.PublishCommentCountDeadLetter(ctx, message.Payload, err.Error()); deadLetterErr != nil {
			return errors.Join(err, deadLetterErr)
		}
		return nil
	}

	// 2. 投影事务失败时执行有限指数退避重试
	delay := commentCountRetryDelay
	var processErr error
	for attempt := 0; attempt < commentCountConsumeAttempts; attempt++ {
		processErr = c.processor.ApplyCommentCountEvent(ctx, event)
		if processErr == nil {
			return nil
		}
		if attempt == commentCountConsumeAttempts-1 {
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
	if err := c.deadLetter.PublishCommentCountDeadLetter(ctx, message.Payload, processErr.Error()); err != nil {
		return errors.Join(fmt.Errorf("处理文章评论计数事件: %w", processErr), fmt.Errorf("投递评论计数死信: %w", err))
	}
	return nil
}
