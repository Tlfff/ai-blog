package consumer

import (
	"context"
	"errors"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

// fakeSubscriber 提供消费者构造所需的订阅接口。
type fakeSubscriber struct{}

// Topic 返回测试主题。
func (fakeSubscriber) Topic() string { return "article-view" }

// Queue 返回测试队列类型。
func (fakeSubscriber) Queue() string { return "test" }

// Subscribe 在单元测试中不启动真实订阅。
func (fakeSubscriber) Subscribe(context.Context, chan<- *stream.Message, chan<- error) error {
	return nil
}

// Close 模拟关闭测试订阅器。
func (fakeSubscriber) Close(context.Context) error { return nil }

// fakeViewProcessor 记录浏览事件消费次数和预设错误。
type fakeViewProcessor struct {
	calls int   // calls 是处理调用次数。
	err   error // err 是预设处理错误。
}

// ConsumeView 记录一次浏览事件处理。
func (f *fakeViewProcessor) ConsumeView(context.Context, article.ViewEvent) error {
	// 1. 累加调用次数并返回预设错误
	f.calls++
	return f.err
}

// fakeDeadLetterPublisher 记录死信投递。
type fakeDeadLetterPublisher struct {
	calls int // calls 是死信投递次数。
}

// PublishDeadLetter 记录一次最终失败消息。
func (f *fakeDeadLetterPublisher) PublishDeadLetter(context.Context, []byte, string) error {
	// 1. 累加死信投递次数
	f.calls++
	return nil
}

// TestArticleViewConsumerRetriesThenPublishesDeadLetter 验证重试耗尽后投递死信并确认源消息。
func TestArticleViewConsumerRetriesThenPublishesDeadLetter(t *testing.T) {
	// 1. 构造持续失败的浏览处理器和死信发布器
	processor := &fakeViewProcessor{err: errors.New("database unavailable")}
	deadLetter := &fakeDeadLetterPublisher{}
	consumer := NewArticleViewConsumer(fakeSubscriber{}, processor, deadLetter)

	// 2. 处理合法事件，内部重试耗尽后由死信接管
	message := &stream.Message{Payload: []byte(`{"event_id":"event","article_id":1,"viewed_at":"2026-09-04T00:00:00Z"}`)}
	if err := consumer.Handle(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if processor.calls != viewConsumeAttempts || deadLetter.calls != 1 {
		t.Fatalf("processor calls = %d, dead letter calls = %d", processor.calls, deadLetter.calls)
	}
}

// TestArticleViewConsumerSendsMalformedPayloadToDeadLetter 验证损坏消息不进入领域处理器。
func TestArticleViewConsumerSendsMalformedPayloadToDeadLetter(t *testing.T) {
	// 1. 处理无法解析的 JSON 负载
	processor := &fakeViewProcessor{}
	deadLetter := &fakeDeadLetterPublisher{}
	consumer := NewArticleViewConsumer(fakeSubscriber{}, processor, deadLetter)
	if err := consumer.Handle(context.Background(), &stream.Message{Payload: []byte(`{`)}); err != nil {
		t.Fatal(err)
	}

	// 2. 损坏消息只投递一次死信
	if processor.calls != 0 || deadLetter.calls != 1 {
		t.Fatalf("processor calls = %d, dead letter calls = %d", processor.calls, deadLetter.calls)
	}
}
