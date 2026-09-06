package article

import (
	"context"
	"errors"
	"time"
)

const (
	CommentCreatedEventType = "comment.created" // CommentCreatedEventType 表示评论已创建事件。
	CommentDeletedEventType = "comment.deleted" // CommentDeletedEventType 表示评论已删除事件。
)

var ErrInvalidCommentCountEvent = errors.New("文章评论计数事件不合法")

// CommentCountEvent 表示文章上下文接收的评论生命周期事实。
type CommentCountEvent struct {
	EventID     string    `json:"event_id"`     // EventID 是消息幂等标识。
	EventType   string    `json:"event_type"`   // EventType 是评论创建或删除事件类型。
	Version     int64     `json:"version"`      // Version 是评论聚合单调版本。
	OccurredAt  time.Time `json:"occurred_at"`  // OccurredAt 是事实发生时间。
	AggregateID uint64    `json:"aggregate_id"` // AggregateID 是评论聚合标识。
	CommentID   uint64    `json:"comment_id"`   // CommentID 是评论标识。
	ArticleID   uint64    `json:"article_id"`   // ArticleID 是文章标识。
	RootID      uint64    `json:"root_id"`      // RootID 是直属根评论标识。
}

// CommentCountRepository 定义文章评论数投影的数据能力。
type CommentCountRepository interface {
	// ApplyCommentCountEvent 原子处理 Inbox、评论状态和文章计数。
	ApplyCommentCountEvent(context.Context, CommentCountEvent) error
}

// CommentCountProcessor 定义评论计数消费者处理能力。
type CommentCountProcessor interface {
	// ApplyCommentCountEvent 幂等应用评论生命周期事件。
	ApplyCommentCountEvent(context.Context, CommentCountEvent) error
}

// CommentCountProjector 校验并应用文章评论数事件。
type CommentCountProjector struct {
	repository CommentCountRepository // repository 提供原子投影事务。
}

// NewCommentCountProjector 创建文章评论数投影器。
func NewCommentCountProjector(repository CommentCountRepository) *CommentCountProjector {
	// 1. 启动阶段拒绝缺少文章投影仓储
	if repository == nil {
		panic("文章评论计数投影器缺少仓储")
	}
	return &CommentCountProjector{repository: repository}
}

// ApplyCommentCountEvent 校验版本化事件后更新文章投影。
func (p *CommentCountProjector) ApplyCommentCountEvent(ctx context.Context, event CommentCountEvent) error {
	// 1. 拒绝无法关联或无法幂等处理的消息
	if event.EventID == "" || event.AggregateID == 0 || event.CommentID == 0 || event.AggregateID != event.CommentID || event.ArticleID == 0 || event.Version <= 0 || event.OccurredAt.IsZero() {
		return ErrInvalidCommentCountEvent
	}
	if event.EventType == CommentCreatedEventType && event.Version != 1 {
		return ErrInvalidCommentCountEvent
	}
	if event.EventType == CommentDeletedEventType && event.Version != 2 {
		return ErrInvalidCommentCountEvent
	}
	if event.EventType != CommentCreatedEventType && event.EventType != CommentDeletedEventType {
		return ErrInvalidCommentCountEvent
	}

	// 2. 由仓储事务处理重复、乱序和计数更新
	return p.repository.ApplyCommentCountEvent(ctx, event)
}

// CommentCountDeadLetterPublisher 定义评论计数消费失败后的死信能力。
type CommentCountDeadLetterPublisher interface {
	// PublishCommentCountDeadLetter 发布原始消息及失败原因。
	PublishCommentCountDeadLetter(context.Context, []byte, string) error
}
