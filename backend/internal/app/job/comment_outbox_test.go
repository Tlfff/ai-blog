package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
)

// fakeCommentOutbox 保存待发布消息和状态变更。
type fakeCommentOutbox struct {
	messages  []comment.OutboxMessage // messages 是待发布消息。
	published int                     // published 是成功标记次数。
	failed    int                     // failed 是失败标记次数。
}

// ListPending 返回测试消息。
func (f *fakeCommentOutbox) ListPending(context.Context, int, time.Time) ([]comment.OutboxMessage, error) {
	// 1. 返回当前待发布消息副本
	return append([]comment.OutboxMessage(nil), f.messages...), nil
}

// MarkPublished 记录发布成功并清空待发布消息。
func (f *fakeCommentOutbox) MarkPublished(context.Context, string, time.Time) error {
	// 1. 模拟数据库完成状态
	f.published++
	f.messages = nil
	return nil
}

// MarkFailed 记录失败但保留消息供下次补发。
func (f *fakeCommentOutbox) MarkFailed(context.Context, string, string, time.Time) error {
	// 1. 模拟重试次数持久化
	f.failed++
	if len(f.messages) > 0 {
		f.messages[0].Attempts++
	}
	return nil
}

// flakyCommentPublisher 首次发布失败，之后成功。
type flakyCommentPublisher struct {
	calls int // calls 是发布调用次数。
}

// Publish 模拟可恢复的 Kafka 发布失败。
func (p *flakyCommentPublisher) Publish(context.Context, comment.IntegrationEvent) error {
	// 1. 第一次返回瞬时错误
	p.calls++
	if p.calls == 1 {
		return errors.New("kafka unavailable")
	}
	return nil
}

// TestCommentOutboxRelayRetriesPublishedFailure 验证发布失败不会丢失 Outbox 且后续可补发。
func TestCommentOutboxRelayRetriesPublishedFailure(t *testing.T) {
	// 1. 第一次发送失败后记录重试并保留消息
	outbox := &fakeCommentOutbox{messages: []comment.OutboxMessage{{Event: comment.IntegrationEvent{EventID: "event-1", EventType: comment.CommentCreatedEventType, Version: 1, AggregateID: 9, CommentID: 9, ArticleID: 3, OccurredAt: time.Now()}}}}
	publisher := &flakyCommentPublisher{}
	relay := NewCommentOutboxRelay(outbox, publisher)
	if err := relay.PublishBatch(context.Background()); err == nil {
		t.Fatal("first publish did not fail")
	}
	if outbox.failed != 1 || outbox.published != 0 || len(outbox.messages) != 1 {
		t.Fatalf("failed=%d published=%d pending=%d", outbox.failed, outbox.published, len(outbox.messages))
	}

	// 2. 下一批补发成功后才标记完成
	if err := relay.PublishBatch(context.Background()); err != nil {
		t.Fatalf("retry publish failed: %v", err)
	}
	if outbox.published != 1 || len(outbox.messages) != 0 {
		t.Fatalf("published=%d pending=%d", outbox.published, len(outbox.messages))
	}
}
