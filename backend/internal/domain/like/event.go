package like

import (
	"context"
	"time"
)

const (
	ArticleLikedEventType   = "article.liked"   // ArticleLikedEventType 表示文章点赞事实已生效。
	ArticleUnlikedEventType = "article.unliked" // ArticleUnlikedEventType 表示文章点赞事实已取消。
)

// IntegrationEvent 是点赞上下文发布的版本化文章点赞事件。
type IntegrationEvent struct {
	EventID     string    `json:"event_id"`     // EventID 是跨投递保持稳定的幂等标识。
	EventType   string    `json:"event_type"`   // EventType 是点赞或取消点赞事件类型。
	Version     int64     `json:"version"`      // Version 是同一点赞关系的单调版本。
	OccurredAt  time.Time `json:"occurred_at"`  // OccurredAt 是点赞事实变更时间。
	AggregateID uint64    `json:"aggregate_id"` // AggregateID 是点赞关系标识。
	LikeID      uint64    `json:"like_id"`      // LikeID 是点赞关系标识。
	ArticleID   uint64    `json:"article_id"`   // ArticleID 是文章标识。
	UserID      uint64    `json:"user_id"`      // UserID 是点赞用户标识。
}

// OutboxMessage 是待发布的点赞集成事件记录。
type OutboxMessage struct {
	Event       IntegrationEvent // Event 是稳定文章点赞事件。
	Attempts    int              // Attempts 是已经失败的发布次数。
	NextAttempt time.Time        // NextAttempt 是允许再次发布的时间。
}

// OutboxRepository 定义点赞 Outbox 发布所需的数据能力。
type OutboxRepository interface {
	// ListPending 查询到期且尚未发布的消息。
	ListPending(context.Context, int, time.Time) ([]OutboxMessage, error)
	// MarkPublished 将消息标记为发布完成。
	MarkPublished(context.Context, string, time.Time) error
	// MarkFailed 记录发布失败并安排下次重试。
	MarkFailed(context.Context, string, string, time.Time) error
}

// EventPublisher 定义文章点赞集成事件的至少一次发布能力。
type EventPublisher interface {
	// Publish 将一条 Outbox 事件发布到消息流。
	Publish(context.Context, IntegrationEvent) error
}
