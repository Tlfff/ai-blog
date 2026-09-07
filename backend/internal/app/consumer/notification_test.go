package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

// fakeNotificationProcessor 记录通知事件处理和瞬时失败次数。
type fakeNotificationProcessor struct {
	calls    int                   // calls 是处理调用次数。
	failures int                   // failures 是成功前失败次数。
	event    like.IntegrationEvent // event 是最近收到的点赞事件。
}

// ConsumeArticleLike 记录事件并按配置模拟瞬时失败。
func (f *fakeNotificationProcessor) ConsumeArticleLike(_ context.Context, event like.IntegrationEvent) error {
	f.calls++
	f.event = event
	if f.calls <= f.failures {
		return errors.New("mongo unavailable")
	}
	return nil
}

// fakeNotificationDeadLetter 记录通知死信发布次数。
type fakeNotificationDeadLetter struct {
	calls int // calls 是死信发布次数。
}

// PublishNotificationDeadLetter 记录一次死信发布。
func (f *fakeNotificationDeadLetter) PublishNotificationDeadLetter(context.Context, []byte, string) error {
	f.calls++
	return nil
}

// TestNotificationConsumerRetriesAndProcessesArticleLike 验证通知消费瞬时失败后重试成功。
func TestNotificationConsumerRetriesAndProcessesArticleLike(t *testing.T) {
	processor := &fakeNotificationProcessor{failures: 2}
	dead := &fakeNotificationDeadLetter{}
	consumer := NewNotificationConsumer(fakeSubscriber{}, processor, dead)
	payload := []byte(`{"event_id":"e1","event_type":"article.liked","version":1,"occurred_at":"` + time.Now().Format(time.RFC3339Nano) + `","article_id":9,"user_id":7}`)
	if err := consumer.Handle(context.Background(), &stream.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if processor.calls != 3 || processor.event.ArticleID != 9 || dead.calls != 0 {
		t.Fatalf("processor=%#v dead=%d", processor, dead.calls)
	}
}

// TestNotificationConsumerDeadLettersMalformedMessage 验证损坏消息只进入一次死信。
func TestNotificationConsumerDeadLettersMalformedMessage(t *testing.T) {
	dead := &fakeNotificationDeadLetter{}
	consumer := NewNotificationConsumer(fakeSubscriber{}, &fakeNotificationProcessor{}, dead)
	if err := consumer.Handle(context.Background(), &stream.Message{Payload: []byte("{")}); err != nil {
		t.Fatal(err)
	}
	if dead.calls != 1 {
		t.Fatalf("dead=%d", dead.calls)
	}
}
