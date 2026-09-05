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
	viewConsumeAttempts = 3                      // viewConsumeAttempts 是浏览事件消费最大尝试次数。
	viewRetryDelay      = 100 * time.Millisecond // viewRetryDelay 是消费重试初始间隔。
)

// ArticleViewConsumer 使用 Leo Stream 处理文章浏览事件。
type ArticleViewConsumer struct {
	subscriber stream.Subscriber               // subscriber 是文章浏览 Kafka 订阅器。
	processor  article.ViewProcessor           // processor 提供幂等浏览投影更新能力。
	deadLetter article.ViewDeadLetterPublisher // deadLetter 提供最终失败消息投递能力。
}

// NewArticleViewConsumer 创建文章浏览事件消费者。
func NewArticleViewConsumer(subscriber stream.Subscriber, processor article.ViewProcessor, deadLetter article.ViewDeadLetterPublisher) *ArticleViewConsumer {
	// 1. 启动阶段拒绝缺少订阅、处理或死信能力
	if subscriber == nil || processor == nil || deadLetter == nil {
		panic("文章浏览消费者缺少必要依赖")
	}
	return &ArticleViewConsumer{subscriber: subscriber, processor: processor, deadLetter: deadLetter}
}

// Subscriber 返回 Leo Stream 使用的文章浏览订阅器。
func (c *ArticleViewConsumer) Subscriber() (stream.Subscriber, error) {
	// 1. 复用启动阶段创建并校验的 Kafka 订阅器
	return c.subscriber, nil
}

// Handle 解析、重试并处理文章浏览事件。
func (c *ArticleViewConsumer) Handle(ctx context.Context, message *stream.Message) error {
	// 1. 解析稳定 JSON 事件，损坏消息直接进入死信主题
	var event article.ViewEvent
	if err := json.Unmarshal(message.Payload, &event); err != nil {
		if deadLetterErr := c.deadLetter.PublishDeadLetter(ctx, message.Payload, err.Error()); deadLetterErr != nil {
			return errors.Join(err, deadLetterErr)
		}
		return nil
	}

	// 2. 对瞬时依赖失败执行有限指数退避重试
	delay := viewRetryDelay
	var processErr error
	for attempt := 0; attempt < viewConsumeAttempts; attempt++ {
		processErr = c.processor.ConsumeView(ctx, event)
		if processErr == nil {
			return nil
		}
		if attempt == viewConsumeAttempts-1 {
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

	// 3. 重试耗尽后投递死信；死信成功即确认源消息，失败则由 Leo Nack
	if err := c.deadLetter.PublishDeadLetter(ctx, message.Payload, processErr.Error()); err != nil {
		return errors.Join(fmt.Errorf("处理文章浏览事件: %w", processErr), fmt.Errorf("投递文章浏览死信: %w", err))
	}
	return nil
}
