package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

const notificationConsumeAttempts = 3

// NotificationConsumer 消费文章点赞事件并创建通知。
type NotificationConsumer struct {
	subscriber stream.Subscriber                // subscriber 是通知独立消费组订阅器。
	processor  notification.Processor           // processor 提供通知领域生成能力。
	deadLetter notification.DeadLetterPublisher // deadLetter 提供最终失败消息投递能力。
}

// NewNotificationConsumer 创建通知消费者。
func NewNotificationConsumer(subscriber stream.Subscriber, processor notification.Processor, deadLetter notification.DeadLetterPublisher) *NotificationConsumer {
	// 1. 启动阶段拒绝缺少订阅、处理或死信依赖
	if subscriber == nil || processor == nil || deadLetter == nil {
		panic("通知消费者缺少必要依赖")
	}
	return &NotificationConsumer{subscriber: subscriber, processor: processor, deadLetter: deadLetter}
}

// Subscriber 返回通知独立消费组订阅器。
func (c *NotificationConsumer) Subscriber() (stream.Subscriber, error) {
	// 1. 返回启动阶段已校验的订阅器
	return c.subscriber, nil
}

// Handle 解析文章点赞事件并有限重试通知生成。
func (c *NotificationConsumer) Handle(ctx context.Context, message *stream.Message) error {
	// 1. 损坏消息直接进入通知死信
	var event like.IntegrationEvent
	if err := json.Unmarshal(message.Payload, &event); err != nil {
		return c.deadLetterResult(ctx, message.Payload, err)
	}

	// 2. MongoDB 或快照查询瞬时失败时有限指数退避重试
	var processErr error
	for attempt := 0; attempt < notificationConsumeAttempts; attempt++ {
		processErr = c.processor.ConsumeArticleLike(ctx, event)
		if processErr == nil {
			return nil
		}
		if attempt < notificationConsumeAttempts-1 {
			timer := time.NewTimer(time.Duration(1<<attempt) * 100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.Join(processErr, ctx.Err())
			case <-timer.C:
			}
		}
	}

	// 3. 重试耗尽后死信成功即确认源消息
	return c.deadLetterResult(ctx, message.Payload, processErr)
}

// deadLetterResult 发布通知消费失败的原始消息。
func (c *NotificationConsumer) deadLetterResult(ctx context.Context, payload []byte, cause error) error {
	// 1. 死信失败时返回组合错误触发 Leo Nack
	if err := c.deadLetter.PublishNotificationDeadLetter(ctx, payload, cause.Error()); err != nil {
		return errors.Join(fmt.Errorf("生成文章点赞通知: %w", cause), fmt.Errorf("发布通知死信: %w", err))
	}
	return nil
}
