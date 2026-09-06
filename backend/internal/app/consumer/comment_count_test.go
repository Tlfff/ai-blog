package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

// fakeCommentCountProcessor 模拟前两次失败后成功的投影处理器。
type fakeCommentCountProcessor struct {
	calls       int   // calls 是处理次数。
	failures    int   // failures 是成功前失败次数。
	alwaysError error // alwaysError 是持续返回的错误。
}

// ApplyCommentCountEvent 记录调用并按配置返回错误。
func (f *fakeCommentCountProcessor) ApplyCommentCountEvent(context.Context, article.CommentCountEvent) error {
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

// fakeCommentCountDeadLetter 记录死信负载。
type fakeCommentCountDeadLetter struct {
	calls int // calls 是死信发布次数。
}

// PublishCommentCountDeadLetter 记录死信发布。
func (f *fakeCommentCountDeadLetter) PublishCommentCountDeadLetter(context.Context, []byte, string) error {
	// 1. 累加死信次数
	f.calls++
	return nil
}

// TestCommentCountConsumerRetriesTransientFailure 验证投影失败由同一消息重试后成功。
func TestCommentCountConsumerRetriesTransientFailure(t *testing.T) {
	// 1. 前两次失败，第三次成功时不进入死信
	processor := &fakeCommentCountProcessor{failures: 2}
	deadLetter := &fakeCommentCountDeadLetter{}
	consumer := NewCommentCountConsumer(fakeSubscriber{}, processor, deadLetter)
	payload := []byte(`{"event_id":"event-1","event_type":"comment.created","version":1,"aggregate_id":9,"occurred_at":"` + time.Now().Format(time.RFC3339Nano) + `","comment_id":9,"article_id":4,"root_id":0}`)
	if err := consumer.Handle(context.Background(), &stream.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if processor.calls != 3 || deadLetter.calls != 0 {
		t.Fatalf("processor calls=%d dead letters=%d", processor.calls, deadLetter.calls)
	}
}

// TestCommentCountConsumerPublishesDeadLetterAfterRetries 验证重试耗尽后进入死信。
func TestCommentCountConsumerPublishesDeadLetterAfterRetries(t *testing.T) {
	// 1. 持续失败的合法消息最终发布一次死信
	processor := &fakeCommentCountProcessor{alwaysError: errors.New("database unavailable")}
	deadLetter := &fakeCommentCountDeadLetter{}
	consumer := NewCommentCountConsumer(fakeSubscriber{}, processor, deadLetter)
	payload := []byte(`{"event_id":"event-2","event_type":"comment.deleted","version":2,"aggregate_id":9,"occurred_at":"` + time.Now().Format(time.RFC3339Nano) + `","comment_id":9,"article_id":4,"root_id":0}`)
	if err := consumer.Handle(context.Background(), &stream.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if processor.calls != commentCountConsumeAttempts || deadLetter.calls != 1 {
		t.Fatalf("processor calls=%d dead letters=%d", processor.calls, deadLetter.calls)
	}
}
