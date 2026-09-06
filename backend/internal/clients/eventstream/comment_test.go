package eventstream

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

// recordingCommentPublisher 记录评论事件发布消息。
type recordingCommentPublisher struct {
	messages []*stream.Message // messages 是实际交给基础发布器的消息。
}

// Topic 返回评论事件测试主题。
func (recordingCommentPublisher) Topic() string {
	// 1. 返回固定测试主题
	return "comment-event-test"
}

// Queue 返回评论事件测试队列类型。
func (recordingCommentPublisher) Queue() string {
	// 1. 返回固定测试队列类型
	return "comment-event-test"
}

// Publish 保存实际发布的评论事件消息。
func (p *recordingCommentPublisher) Publish(_ context.Context, messages ...*stream.Message) (stream.Result, error) {
	// 1. 复制消息切片以便断言发布结果
	p.messages = append(p.messages, messages...)
	return nil, nil
}

// Close 模拟关闭评论事件发布器。
func (*recordingCommentPublisher) Close(context.Context) error {
	// 1. 测试发布器无需释放资源
	return nil
}

// TestCommentEventPublisherPublishesIntegrationEvent 验证 Publisher 实际编码并发布 Outbox 事件。
func TestCommentEventPublisherPublishesIntegrationEvent(t *testing.T) {
	// 1. 发布包含稳定幂等键和聚合版本的评论事件
	occurredAt := time.Date(2026, time.September, 6, 12, 30, 0, 123456000, time.UTC)
	event := comment.IntegrationEvent{EventID: "comment-event-9-v1", EventType: comment.CommentCreatedEventType, Version: comment.CommentCreatedVersion, OccurredAt: occurredAt, AggregateID: 9, CommentID: 9, ArticleID: 4, RootID: 0}
	underlying := &recordingCommentPublisher{}
	publisher := &CommentEventPublisher{publisher: underlying}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	// 2. 基础发布器收到完整 JSON 事件及事实发生时间
	if len(underlying.messages) != 1 {
		t.Fatalf("published messages = %d, want 1", len(underlying.messages))
	}
	message := underlying.messages[0]
	if !message.Time.Equal(occurredAt) {
		t.Fatalf("message time = %v, want %v", message.Time, occurredAt)
	}
	var published comment.IntegrationEvent
	if err := json.Unmarshal(message.Payload, &published); err != nil {
		t.Fatal(err)
	}
	if published != event {
		t.Fatalf("published event = %#v, want %#v", published, event)
	}
}
