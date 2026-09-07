package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

const (
	likeCountConsumeAttempts = 3                      // likeCountConsumeAttempts 是点赞计数消费最大尝试次数。
	likeCountRetryDelay      = 100 * time.Millisecond // likeCountRetryDelay 是重试初始间隔。
)

// LikeCountConsumer 使用 Leo Stream 路由并维护文章、评论点赞数投影。
type LikeCountConsumer struct {
	subscriber       stream.Subscriber                    // subscriber 是点赞事件订阅器。
	articleProcessor article.LikeCountProcessor           // articleProcessor 提供文章点赞投影能力。
	commentProcessor comment.LikeCountProcessor           // commentProcessor 提供评论点赞投影能力。
	deadLetter       article.LikeCountDeadLetterPublisher // deadLetter 提供最终失败消息投递能力。
}

// NewLikeCountConsumer 创建点赞数消费者。
func NewLikeCountConsumer(subscriber stream.Subscriber, articleProcessor article.LikeCountProcessor, commentProcessor comment.LikeCountProcessor, deadLetter article.LikeCountDeadLetterPublisher) *LikeCountConsumer {
	// 1. 启动阶段拒绝缺少订阅、任一投影或死信依赖
	if subscriber == nil || articleProcessor == nil || commentProcessor == nil || deadLetter == nil {
		panic("点赞计数消费者缺少必要依赖")
	}
	return &LikeCountConsumer{subscriber: subscriber, articleProcessor: articleProcessor, commentProcessor: commentProcessor, deadLetter: deadLetter}
}

// Subscriber 返回 Leo Stream 使用的点赞事件订阅器。
func (c *LikeCountConsumer) Subscriber() (stream.Subscriber, error) {
	// 1. 返回启动阶段已校验的订阅器
	return c.subscriber, nil
}

// Handle 解析、路由并重试处理点赞计数事件。
func (c *LikeCountConsumer) Handle(ctx context.Context, message *stream.Message) error {
	// 1. 先解析点赞上下文稳定事件，以事件类型路由目标上下文
	var event like.IntegrationEvent
	if err := json.Unmarshal(message.Payload, &event); err != nil {
		return c.publishDeadLetter(ctx, message.Payload, err)
	}

	// 2. 投影事务失败时执行有限指数退避重试
	delay := likeCountRetryDelay
	var processErr error
	for attempt := 0; attempt < likeCountConsumeAttempts; attempt++ {
		processErr = c.apply(ctx, event)
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
	return c.publishDeadLetter(ctx, message.Payload, processErr)
}

// apply 按事件类型调用文章或评论上下文投影器。
func (c *LikeCountConsumer) apply(ctx context.Context, event like.IntegrationEvent) error {
	// 1. 文章和评论事件共享传输，但各自只接收本上下文字段
	switch event.EventType {
	case like.ArticleLikedEventType, like.ArticleUnlikedEventType:
		return c.articleProcessor.ApplyLikeCountEvent(ctx, article.LikeCountEvent{EventID: event.EventID, EventType: event.EventType, Version: event.Version, OccurredAt: event.OccurredAt, AggregateID: event.AggregateID, LikeID: event.LikeID, ArticleID: event.ArticleID, UserID: event.UserID})
	case like.CommentLikedEventType, like.CommentUnlikedEventType:
		return c.commentProcessor.ApplyLikeCountEvent(ctx, comment.LikeCountEvent{EventID: event.EventID, EventType: event.EventType, Version: event.Version, OccurredAt: event.OccurredAt, AggregateID: event.AggregateID, LikeID: event.LikeID, CommentID: event.CommentID, UserID: event.UserID})
	default:
		return fmt.Errorf("未知点赞事件类型")
	}
}

// publishDeadLetter 发布原始负载和安全失败原因。
func (c *LikeCountConsumer) publishDeadLetter(ctx context.Context, payload []byte, cause error) error {
	// 1. 死信成功后确认源消息，死信失败则返回组合错误触发 Nack
	if err := c.deadLetter.PublishLikeCountDeadLetter(ctx, payload, cause.Error()); err != nil {
		return errors.Join(fmt.Errorf("处理点赞计数事件: %w", cause), fmt.Errorf("投递点赞计数死信: %w", err))
	}
	return nil
}
