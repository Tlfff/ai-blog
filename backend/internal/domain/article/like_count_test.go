package article

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeLikeCountRepository 记录文章点赞计数事件。
type fakeLikeCountRepository struct {
	events []LikeCountEvent // events 是收到的合法点赞事件。
}

// ApplyLikeCountEvent 记录合法事件。
func (f *fakeLikeCountRepository) ApplyLikeCountEvent(_ context.Context, event LikeCountEvent) error {
	// 1. 保存事件供断言
	f.events = append(f.events, event)
	return nil
}

// TestLikeCountProjectorValidatesIntegrationEvent 验证投影器拒绝无法幂等处理的事件。
func TestLikeCountProjectorValidatesIntegrationEvent(t *testing.T) {
	// 1. 合法点赞事件进入文章投影仓储
	repository := &fakeLikeCountRepository{}
	projector := NewLikeCountProjector(repository)
	event := LikeCountEvent{EventID: "like-9-v1", EventType: ArticleLikedEventType, Version: 1, AggregateID: 9, LikeID: 9, ArticleID: 4, UserID: 7, OccurredAt: time.Now()}
	if err := projector.ApplyLikeCountEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(repository.events) != 1 || repository.events[0].LikeID != 9 {
		t.Fatalf("events = %#v", repository.events)
	}

	// 2. 缺少用户标识或未知事件类型时被拒绝
	event.UserID = 0
	if err := projector.ApplyLikeCountEvent(context.Background(), event); !errors.Is(err, ErrInvalidLikeCountEvent) {
		t.Fatalf("invalid event error = %v", err)
	}
	event.UserID = 7
	event.EventType = "comment.liked"
	if err := projector.ApplyLikeCountEvent(context.Background(), event); !errors.Is(err, ErrInvalidLikeCountEvent) {
		t.Fatalf("unknown event error = %v", err)
	}
}
