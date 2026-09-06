package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
)

// fakeLikeOutbox 保存待发布消息和状态变更。
type fakeLikeOutbox struct {
	messages  []like.OutboxMessage // messages 是待发布消息。
	published int                  // published 是成功标记次数。
	failed    int                  // failed 是失败标记次数。
}

// ListPending 返回测试消息。
func (f *fakeLikeOutbox) ListPending(context.Context, int, time.Time) ([]like.OutboxMessage, error) {
	// 1. 返回当前待发布消息副本
	return append([]like.OutboxMessage(nil), f.messages...), nil
}

// MarkPublished 记录发布成功并清空待发布消息。
func (f *fakeLikeOutbox) MarkPublished(context.Context, string, time.Time) error {
	// 1. 模拟数据库完成状态
	f.published++
	f.messages = nil
	return nil
}

// MarkFailed 记录失败但保留消息供下次补发。
func (f *fakeLikeOutbox) MarkFailed(context.Context, string, string, time.Time) error {
	// 1. 模拟重试次数持久化
	f.failed++
	if len(f.messages) > 0 {
		f.messages[0].Attempts++
	}
	return nil
}

// flakyLikePublisher 首次发布失败，之后成功。
type flakyLikePublisher struct {
	calls int // calls 是发布调用次数。
}

// Publish 模拟可恢复的 Kafka 发布失败。
func (p *flakyLikePublisher) Publish(context.Context, like.IntegrationEvent) error {
	// 1. 第一次返回瞬时错误
	p.calls++
	if p.calls == 1 {
		return errors.New("kafka unavailable")
	}
	return nil
}

// TestLikeOutboxRelayRetriesPublishedFailure 验证发布失败不会丢失 Outbox 且后续可补发。
func TestLikeOutboxRelayRetriesPublishedFailure(t *testing.T) {
	// 1. 第一次发送失败后记录重试并保留消息
	outbox := &fakeLikeOutbox{messages: []like.OutboxMessage{{Event: like.IntegrationEvent{EventID: "event-1", EventType: like.ArticleLikedEventType, Version: 1, AggregateID: 9, LikeID: 9, UserID: 7, ArticleID: 3, OccurredAt: time.Now()}}}}
	publisher := &flakyLikePublisher{}
	relay := NewLikeOutboxRelay(outbox, publisher)
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
	if outbox.published != 1 || len(outbox.messages) != 0 || publisher.calls != 2 {
		t.Fatalf("published=%d pending=%d calls=%d", outbox.published, len(outbox.messages), publisher.calls)
	}
}
