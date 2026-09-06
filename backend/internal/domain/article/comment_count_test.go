package article

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeCommentCountRepository 记录文章评论计数事件。
type fakeCommentCountRepository struct {
	events []CommentCountEvent // events 是已提交的评论事件。
	err    error               // err 是预设仓储错误。
}

// ApplyCommentCountEvent 原子应用评论状态与文章计数投影。
func (f *fakeCommentCountRepository) ApplyCommentCountEvent(_ context.Context, event CommentCountEvent) error {
	// 1. 记录事件并返回预设错误
	f.events = append(f.events, event)
	return f.err
}

// TestCommentCountProjectorValidatesAndAppliesEvent 验证投影器只接受完整版本化事件。
func TestCommentCountProjectorValidatesAndAppliesEvent(t *testing.T) {
	// 1. 合法事件交给文章仓储原子处理
	repository := &fakeCommentCountRepository{}
	projector := NewCommentCountProjector(repository)
	event := CommentCountEvent{EventID: "event-1", EventType: CommentCreatedEventType, Version: 1, AggregateID: 9, CommentID: 9, ArticleID: 3, OccurredAt: time.Now()}
	if err := projector.ApplyCommentCountEvent(context.Background(), event); err != nil {
		t.Fatalf("应用事件失败: %v", err)
	}
	if len(repository.events) != 1 || repository.events[0].CommentID != 9 {
		t.Fatalf("events = %#v", repository.events)
	}

	// 2. 缺少幂等标识的事件被拒绝
	event.EventID = ""
	if err := projector.ApplyCommentCountEvent(context.Background(), event); !errors.Is(err, ErrInvalidCommentCountEvent) {
		t.Fatalf("非法事件错误 = %v", err)
	}
}
