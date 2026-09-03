package consumer

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/leo/leo/log"
	"codeup.aliyun.com/qimao/leo/leo/stream"
	"codeup.aliyun.com/qimao/leo/leo/stream/kafka"
	"context"
	kafka2 "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// BlogConsumer 表示博客消息消费者。
type BlogConsumer struct {
	svc *book.HelloworldService
}

// NewConsumer 创建博客消息消费者。
func NewConsumer(svc *book.HelloworldService) *BlogConsumer {
	return &BlogConsumer{
		svc: svc,
	}
}

// Subscriber 创建博客消息订阅器。
func (b *BlogConsumer) Subscriber() (stream.Subscriber, error) {
	topic := "event-tracking"
	factory := func() (*kafka2.Consumer, error) {
		return kafka2.NewConsumer(&kafka2.ConfigMap{
			"api.version.request":       "true",
			"auto.offset.reset":         "latest",
			"heartbeat.interval.ms":     3000,
			"session.timeout.ms":        30000,
			"max.poll.interval.ms":      120000,
			"fetch.max.bytes":           1024000,
			"max.partition.fetch.bytes": 256000,
			"bootstrap.servers":         "localhost:9092",
			"group.id":                  "TestSubscriber",
		})
	}
	return kafka.NewSubscriber(topic, factory, kafka.NackHandler(
		func(msg *stream.Message) {
			log.Error("nack msg: ", string(msg.Payload))
		},
	))
}

// Handle 处理博客消息。
func (b *BlogConsumer) Handle(ctx context.Context, msg *stream.Message) error {
	log.Debug("message", string(msg.Payload))
	return nil
}
