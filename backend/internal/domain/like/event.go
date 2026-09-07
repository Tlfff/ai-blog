package like

import (
	"context"
	"time"
)

const (
	ArticleLikedEventType   = "article.liked"   // ArticleLikedEventType 表示文章点赞事实已生效。
	ArticleUnlikedEventType = "article.unliked" // ArticleUnlikedEventType 表示文章点赞事实已取消。
	CommentLikedEventType   = "comment.liked"   // CommentLikedEventType 表示评论点赞事实已生效。
	CommentUnlikedEventType = "comment.unliked" // CommentUnlikedEventType 表示评论点赞事实已取消。
)

// IntegrationEvent 是点赞上下文发布的版本化文章或评论点赞事件。
type IntegrationEvent struct {
	EventID     string    `json:"event_id"`             // EventID 是跨投递保持稳定的幂等标识。
	EventType   string    `json:"event_type"`           // EventType 是点赞或取消点赞事件类型。
	Version     int64     `json:"version"`              // Version 是同一点赞关系的单调版本。
	OccurredAt  time.Time `json:"occurred_at"`          // OccurredAt 是点赞事实变更时间。
	AggregateID uint64    `json:"aggregate_id"`         // AggregateID 是点赞关系标识。
	LikeID      uint64    `json:"like_id"`              // LikeID 是点赞关系标识。
	ArticleID   uint64    `json:"article_id,omitempty"` // ArticleID 是文章标识，评论事件为 0。
	CommentID   uint64    `json:"comment_id,omitempty"` // CommentID 是评论标识，文章事件为 0。
	UserID      uint64    `json:"user_id"`              // UserID 是点赞用户标识。
}

// OutboxMessage 是待发布的点赞集成事件记录。
type OutboxMessage struct {
	Event       IntegrationEvent // Event 是稳定的文章或评论点赞事件。
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

// EventPublisher 定义点赞集成事件的至少一次发布能力。
type EventPublisher interface {
	// Publish 将一条 Outbox 事件发布到消息流。
	Publish(context.Context, IntegrationEvent) error
}

// CommentOutboxRepository 定义评论点赞 Outbox 发布所需的数据能力。
type CommentOutboxRepository interface {
	// ListPendingCommentLikes 查询到期且尚未发布的评论点赞消息。
	ListPendingCommentLikes(context.Context, int, time.Time) ([]OutboxMessage, error)
	// MarkCommentLikePublished 将评论点赞消息标记为发布完成。
	MarkCommentLikePublished(context.Context, string, time.Time) error
	// MarkCommentLikeFailed 记录评论点赞发布失败并安排下次重试。
	MarkCommentLikeFailed(context.Context, string, string, time.Time) error
}
