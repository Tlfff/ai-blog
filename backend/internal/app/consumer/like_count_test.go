package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

// fakeLikeCountProcessor 模拟瞬时或持续失败的点赞投影处理器。
type fakeLikeCountProcessor struct {
	calls       int   // calls 是处理次数。
	failures    int   // failures 是成功前失败次数。
	alwaysError error // alwaysError 是持续返回的错误。
}

// ApplyLikeCountEvent 记录调用并按配置返回错误。
func (f *fakeLikeCountProcessor) ApplyLikeCountEvent(context.Context, article.LikeCountEvent) error {
	// 1. 根据测试场景返回瞬时或持续错误
	f.calls++
	if f.alwaysError != nil {
		return f.alwaysError
	}
	if f.calls <= f.failures {
		return errors.New("mysql unavailable")
	}
	return nil
}

// fakeLikeCountDeadLetter 记录点赞死信负载。
type fakeLikeCountDeadLetter struct {
	calls int // calls 是死信发布次数。
}

// PublishLikeCountDeadLetter 记录死信发布。
func (f *fakeLikeCountDeadLetter) PublishLikeCountDeadLetter(context.Context, []byte, string) error {
	// 1. 累加死信次数
	f.calls++
	return nil
}

// TestLikeCountConsumerRetriesTransientFailure 验证投影失败由同一消息重试后成功。
func TestLikeCountConsumerRetriesTransientFailure(t *testing.T) {
	// 1. 前两次失败，第三次成功时不进入死信
	processor := &fakeLikeCountProcessor{failures: 2}
	deadLetter := &fakeLikeCountDeadLetter{}
	consumer := NewLikeCountConsumer(fakeSubscriber{}, processor, deadLetter)
	payload := []byte(`{"event_id":"event-1","event_type":"article.liked","version":1,"aggregate_id":9,"occurred_at":"` + time.Now().Format(time.RFC3339Nano) + `","like_id":9,"article_id":4,"user_id":7}`)
	if err := consumer.Handle(context.Background(), &stream.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if processor.calls != 3 || deadLetter.calls != 0 {
		t.Fatalf("processor calls=%d dead letters=%d", processor.calls, deadLetter.calls)
	}
}

// TestLikeCountConsumerPublishesDeadLetterAfterRetries 验证重试耗尽后进入死信。
func TestLikeCountConsumerPublishesDeadLetterAfterRetries(t *testing.T) {
	// 1. 持续失败的合法消息最终发布一次死信
	processor := &fakeLikeCountProcessor{alwaysError: errors.New("database unavailable")}
	deadLetter := &fakeLikeCountDeadLetter{}
	consumer := NewLikeCountConsumer(fakeSubscriber{}, processor, deadLetter)
	payload := []byte(`{"event_id":"event-2","event_type":"article.unliked","version":2,"aggregate_id":9,"occurred_at":"` + time.Now().Format(time.RFC3339Nano) + `","like_id":9,"article_id":4,"user_id":7}`)
	if err := consumer.Handle(context.Background(), &stream.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if processor.calls != likeCountConsumeAttempts || deadLetter.calls != 1 {
		t.Fatalf("processor calls=%d dead letters=%d", processor.calls, deadLetter.calls)
	}
}
