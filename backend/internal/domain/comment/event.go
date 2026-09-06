package comment

import (
	"context"
	"time"
)

const (
	CommentCreatedEventType = "comment.created" // CommentCreatedEventType 表示评论已创建事件。
	CommentDeletedEventType = "comment.deleted" // CommentDeletedEventType 表示评论已删除事件。
	CommentCreatedVersion   = int64(1)          // CommentCreatedVersion 是创建事件的聚合版本。
	CommentDeletedVersion   = int64(2)          // CommentDeletedVersion 是删除事件的聚合版本。
)

// IntegrationEvent 是评论上下文发布的版本化集成事件。
type IntegrationEvent struct {
	EventID     string    `json:"event_id"`     // EventID 是跨投递保持稳定的幂等标识。
	EventType   string    `json:"event_type"`   // EventType 是稳定事件类型。
	Version     int64     `json:"version"`      // Version 是同一评论聚合的单调版本。
	OccurredAt  time.Time `json:"occurred_at"`  // OccurredAt 是事实发生时间。
	AggregateID uint64    `json:"aggregate_id"` // AggregateID 是评论聚合标识。
	CommentID   uint64    `json:"comment_id"`   // CommentID 是评论标识。
	ArticleID   uint64    `json:"article_id"`   // ArticleID 是所属文章标识。
	RootID      uint64    `json:"root_id"`      // RootID 是直属根评论标识，主评论为0。
}

// OutboxMessage 是待发布的评论集成事件记录。
type OutboxMessage struct {
	Event       IntegrationEvent // Event 是稳定评论集成事件。
	Attempts    int              // Attempts 是已经失败的发布次数。
	NextAttempt time.Time        // NextAttempt 是允许再次发布的时间。
}

// OutboxRepository 定义评论 Outbox 发布所需的数据能力。
type OutboxRepository interface {
	// ListPending 查询到期且尚未发布的消息。
	ListPending(context.Context, int, time.Time) ([]OutboxMessage, error)
	// MarkPublished 将消息标记为发布完成。
	MarkPublished(context.Context, string, time.Time) error
	// MarkFailed 记录发布失败并安排下次重试。
	MarkFailed(context.Context, string, string, time.Time) error
}

// EventPublisher 定义评论集成事件的至少一次发布能力。
type EventPublisher interface {
	// Publish 将一条 Outbox 事件发布到消息流。
	Publish(context.Context, IntegrationEvent) error
}
