package comment

import (
	"context"
	"time"
)

const (
	CommentLikedEventType   = "comment.liked"   // CommentLikedEventType 表示评论点赞事实已生效。
	CommentUnlikedEventType = "comment.unliked" // CommentUnlikedEventType 表示评论点赞事实已取消。
)

// LikeCountEvent 是评论上下文消费的版本化点赞计数事件。
type LikeCountEvent struct {
	EventID     string    `json:"event_id"`     // EventID 是跨投递保持稳定的幂等标识。
	EventType   string    `json:"event_type"`   // EventType 是点赞或取消点赞事件类型。
	Version     int64     `json:"version"`      // Version 是同一点赞关系的单调版本。
	OccurredAt  time.Time `json:"occurred_at"`  // OccurredAt 是点赞事实变更时间。
	AggregateID uint64    `json:"aggregate_id"` // AggregateID 是点赞关系标识。
	LikeID      uint64    `json:"like_id"`      // LikeID 是点赞关系标识。
	CommentID   uint64    `json:"comment_id"`   // CommentID 是评论标识。
	UserID      uint64    `json:"user_id"`      // UserID 是点赞用户标识。
}

// LikeCountRepository 定义评论点赞计数投影与事实重建能力。
type LikeCountRepository interface {
	// ApplyLikeCountEvent 原子应用幂等、乱序安全的评论点赞事件。
	ApplyLikeCountEvent(context.Context, LikeCountEvent) error
	// RebuildLikeCountProjection 从点赞事实重建关系投影和评论点赞数。
	RebuildLikeCountProjection(context.Context) error
}

// LikeCountProcessor 定义评论点赞计数事件处理能力。
type LikeCountProcessor interface {
	// ApplyLikeCountEvent 校验并应用评论点赞计数事件。
	ApplyLikeCountEvent(context.Context, LikeCountEvent) error
}

// LikeCountRebuilder 定义评论点赞计数投影重建能力。
type LikeCountRebuilder interface {
	// RebuildCommentLikeCount 从点赞事实重建评论点赞数。
	RebuildCommentLikeCount(context.Context) error
}

// LikeCountProjector 校验并维护评论点赞数最终一致投影。
type LikeCountProjector struct {
	repository LikeCountRepository // repository 提供 Inbox、关系投影和评论计数事务。
}

// NewLikeCountProjector 创建评论点赞数投影器。
func NewLikeCountProjector(repository LikeCountRepository) *LikeCountProjector {
	// 1. 启动阶段拒绝缺少评论投影仓储
	if repository == nil {
		panic("评论点赞计数投影器缺少仓储")
	}
	return &LikeCountProjector{repository: repository}
}

// ApplyLikeCountEvent 校验事件后维护评论点赞数。
func (p *LikeCountProjector) ApplyLikeCountEvent(ctx context.Context, event LikeCountEvent) error {
	// 1. 拒绝缺少幂等键、关系版本或评论标识的事件
	if event.EventID == "" || event.LikeID == 0 || event.AggregateID != event.LikeID || event.CommentID == 0 || event.UserID == 0 || event.Version <= 0 || event.OccurredAt.IsZero() {
		return ErrInvalidLikeCountEvent
	}
	if event.EventType != CommentLikedEventType && event.EventType != CommentUnlikedEventType {
		return ErrInvalidLikeCountEvent
	}

	// 2. 由仓储事务处理重复、乱序和计数更新
	return p.repository.ApplyLikeCountEvent(ctx, event)
}

// RebuildCommentLikeCount 从 MySQL 点赞事实重建评论计数与关系投影。
func (p *LikeCountProjector) RebuildCommentLikeCount(ctx context.Context) error {
	// 1. 重建事务由评论仓储统一保证计数与关系投影一致
	return p.repository.RebuildLikeCountProjection(ctx)
}
