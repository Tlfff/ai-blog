package eventstream

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

// recordingLikePublisher 记录文章点赞事件发布消息。
type recordingLikePublisher struct {
	messages []*stream.Message // messages 是实际交给基础发布器的消息。
}

// Topic 返回点赞事件测试主题。
func (recordingLikePublisher) Topic() string { return "article-like-event-test" }

// Queue 返回点赞事件测试队列类型。
func (recordingLikePublisher) Queue() string { return "article-like-event-test" }

// Publish 保存实际发布的点赞事件消息。
func (p *recordingLikePublisher) Publish(_ context.Context, messages ...*stream.Message) (stream.Result, error) {
	// 1. 复制消息切片以便断言发布结果
	p.messages = append(p.messages, messages...)
	return nil, nil
}

// Close 模拟关闭点赞事件发布器。
func (*recordingLikePublisher) Close(context.Context) error { return nil }

// TestLikeEventPublisherPublishesIntegrationEvent 验证 Publisher 实际编码并发布 Outbox 事件。
func TestLikeEventPublisherPublishesIntegrationEvent(t *testing.T) {
	// 1. 发布包含稳定幂等键和聚合版本的点赞事件
	occurredAt := time.Date(2026, time.September, 6, 12, 30, 0, 123456000, time.UTC)
	event := like.IntegrationEvent{EventID: "like-event-9-v1", EventType: like.ArticleLikedEventType, Version: 1, OccurredAt: occurredAt, AggregateID: 9, LikeID: 9, ArticleID: 4, UserID: 7}
	underlying := &recordingLikePublisher{}
	publisher := &LikeEventPublisher{publisher: underlying}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	// 2. 基础发布器收到完整 JSON 事件及事实发生时间
	if len(underlying.messages) != 1 {
		t.Fatalf("published messages=%d want=1", len(underlying.messages))
	}
	message := underlying.messages[0]
	if !message.Time.Equal(occurredAt) {
		t.Fatalf("message time=%v want=%v", message.Time, occurredAt)
	}
	var published like.IntegrationEvent
	if err := json.Unmarshal(message.Payload, &published); err != nil {
		t.Fatal(err)
	}
	if published != event {
		t.Fatalf("published event=%#v want=%#v", published, event)
	}
}
