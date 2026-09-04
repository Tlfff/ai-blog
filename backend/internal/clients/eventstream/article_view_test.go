package eventstream

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/leo/leo/stream"
)

// fakePublisher 记录 Kafka 发布和关闭行为。
type fakePublisher struct {
	published int   // published 是成功接收的消息数量。
	closeErr  error // closeErr 是预设关闭错误。
}

// Topic 返回测试主题。
func (fakePublisher) Topic() string { return "test" }

// Queue 返回测试队列类型。
func (fakePublisher) Queue() string { return "test" }

// Publish 记录测试消息发布。
func (f *fakePublisher) Publish(context.Context, ...*stream.Message) (stream.Result, error) {
	// 1. 累加成功发布数量
	f.published++
	return nil, nil
}

// Close 返回预设 Kafka 关闭错误。
func (f *fakePublisher) Close(context.Context) error {
	// 1. 将关闭结果交给 Runner 返回
	return f.closeErr
}

// TestArticleViewPublisherDrainsQueueAndReturnsCloseError 验证退出时排空队列并报告关闭错误。
func TestArticleViewPublisherDrainsQueueAndReturnsCloseError(t *testing.T) {
	// 1. 在 Runner 启动前接受一条文章浏览事件
	closeErr := errors.New("close failed")
	kafkaPublisher := &fakePublisher{closeErr: closeErr}
	publisher := &ArticleViewPublisher{publisher: kafkaPublisher, queue: make(chan *stream.Message, 1)}
	if err := publisher.PublishView(context.Background(), article.ViewEvent{EventID: "event", ArticleID: 1, ViewedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	// 2. 已取消上下文触发排空和 Kafka 关闭
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := publisher.Run(ctx)
	if kafkaPublisher.published != 1 || !errors.Is(err, closeErr) {
		t.Fatalf("published = %d, error = %v", kafkaPublisher.published, err)
	}
}
