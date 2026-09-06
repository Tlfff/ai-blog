package article

import (
	"context"
	"errors"
	"time"
)

const (
	ArticleLikedEventType   = "article.liked"   // ArticleLikedEventType 表示文章点赞事实已生效。
	ArticleUnlikedEventType = "article.unliked" // ArticleUnlikedEventType 表示文章点赞事实已取消。
)

var ErrInvalidLikeCountEvent = errors.New("文章点赞计数事件不合法")

// LikeCountEvent 表示文章上下文接收的点赞关系最终状态。
type LikeCountEvent struct {
	EventID     string    `json:"event_id"`     // EventID 是消息幂等标识。
	EventType   string    `json:"event_type"`   // EventType 是点赞或取消点赞事件类型。
	Version     int64     `json:"version"`      // Version 是点赞关系单调版本。
	OccurredAt  time.Time `json:"occurred_at"`  // OccurredAt 是事实发生时间。
	AggregateID uint64    `json:"aggregate_id"` // AggregateID 是点赞关系标识。
	LikeID      uint64    `json:"like_id"`      // LikeID 是点赞关系标识。
	ArticleID   uint64    `json:"article_id"`   // ArticleID 是文章标识。
	UserID      uint64    `json:"user_id"`      // UserID 是点赞用户标识。
}

// LikeCountRepository 定义文章点赞数投影的数据能力。
type LikeCountRepository interface {
	// ApplyLikeCountEvent 原子处理 Inbox、点赞状态和文章计数。
	ApplyLikeCountEvent(context.Context, LikeCountEvent) error
}

// LikeCountProcessor 定义点赞计数消费者处理能力。
type LikeCountProcessor interface {
	// ApplyLikeCountEvent 幂等应用文章点赞状态事件。
	ApplyLikeCountEvent(context.Context, LikeCountEvent) error
}

// LikeCountProjector 校验并应用文章点赞数事件。
type LikeCountProjector struct {
	repository LikeCountRepository // repository 提供原子投影事务。
}

// NewLikeCountProjector 创建文章点赞数投影器。
func NewLikeCountProjector(repository LikeCountRepository) *LikeCountProjector {
	// 1. 启动阶段拒绝缺少文章投影仓储
	if repository == nil {
		panic("文章点赞计数投影器缺少仓储")
	}
	return &LikeCountProjector{repository: repository}
}

// ApplyLikeCountEvent 校验版本化事件后更新文章投影。
func (p *LikeCountProjector) ApplyLikeCountEvent(ctx context.Context, event LikeCountEvent) error {
	// 1. 拒绝无法关联或无法幂等处理的消息
	if event.EventID == "" || event.AggregateID == 0 || event.LikeID == 0 || event.AggregateID != event.LikeID || event.ArticleID == 0 || event.UserID == 0 || event.Version <= 0 || event.OccurredAt.IsZero() {
		return ErrInvalidLikeCountEvent
	}
	if event.EventType != ArticleLikedEventType && event.EventType != ArticleUnlikedEventType {
		return ErrInvalidLikeCountEvent
	}

	// 2. 由仓储事务处理重复、乱序和计数更新
	return p.repository.ApplyLikeCountEvent(ctx, event)
}

// LikeCountDeadLetterPublisher 定义点赞计数消费失败后的死信能力。
type LikeCountDeadLetterPublisher interface {
	// PublishLikeCountDeadLetter 发布原始消息及失败原因。
	PublishLikeCountDeadLetter(context.Context, []byte, string) error
}
