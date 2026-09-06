package eventstream

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/leo/leo/stream"
	leokafka "codeup.aliyun.com/qimao/leo/leo/stream/kafka"
	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// CommentEventPublisher 同步发布评论 Outbox 事件。
type CommentEventPublisher struct {
	publisher stream.Publisher // publisher 是 Leo Kafka 发布器。
	mutex     sync.RWMutex     // mutex 协调发布与关闭。
	closed    bool             // closed 表示发布器已关闭。
}

// CommentEventSubscriber 包装评论事件 Kafka 订阅器。
type CommentEventSubscriber struct {
	stream.Subscriber // Subscriber 提供 Leo Stream 订阅能力。
}

// CommentEventDeadLetterPublisher 发布文章评论计数消费死信。
type CommentEventDeadLetterPublisher struct {
	publisher stream.Publisher // publisher 是 Leo Kafka 死信发布器。
	mutex     sync.RWMutex     // mutex 协调发布与关闭。
	closed    bool             // closed 表示发布器已关闭。
}

// NewCommentEventPublisher 创建评论事件 Kafka 发布器。
func NewCommentEventPublisher(config *conf.Config) (*CommentEventPublisher, error) {
	// 1. 从版本化配置字段创建同步发布器
	publisher, err := newCommentPublisher(config.GetData().GetKafka().GetProducer().GetCommentEvent(), "评论事件")
	if err != nil {
		return nil, err
	}
	return &CommentEventPublisher{publisher: publisher}, nil
}

// NewCommentEventSubscriber 创建文章评论计数 Kafka 订阅器。
func NewCommentEventSubscriber(config *conf.Config) (*CommentEventSubscriber, error) {
	// 1. 从版本化配置字段创建手动提交订阅器
	cfg := config.GetData().GetKafka().GetConsumer().GetCommentEvent()
	if cfg.GetBootstrapServers() == "" || cfg.GetTopic() == "" || cfg.GetGroupId() == "" {
		return nil, fmt.Errorf("缺少评论事件 Kafka consumer 配置")
	}
	factory := func() (*confluent.Consumer, error) {
		values := confluent.ConfigMap{"bootstrap.servers": cfg.GetBootstrapServers(), "group.id": cfg.GetGroupId(), "auto.offset.reset": "earliest", "enable.auto.commit": false}
		for key, value := range cfg.GetConfigMap() {
			values[key] = value
		}
		return confluent.NewConsumer(&values)
	}
	subscriber, err := leokafka.NewSubscriber(cfg.GetTopic(), factory, leokafka.AutoCommit(false))
	if err != nil {
		return nil, err
	}
	return &CommentEventSubscriber{Subscriber: subscriber}, nil
}

// NewCommentEventDeadLetterPublisher 创建评论计数死信发布器。
func NewCommentEventDeadLetterPublisher(config *conf.Config) (*CommentEventDeadLetterPublisher, error) {
	// 1. 从版本化配置字段创建死信发布器
	publisher, err := newCommentPublisher(config.GetData().GetKafka().GetProducer().GetCommentEventDeadLetter(), "评论事件死信")
	if err != nil {
		return nil, err
	}
	return &CommentEventDeadLetterPublisher{publisher: publisher}, nil
}

// Publish 同步等待 Kafka 接受评论事件。
func (p *CommentEventPublisher) Publish(ctx context.Context, event comment.IntegrationEvent) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if p.closed {
		return stream.ErrPublisherClosed
	}
	// 1. Outbox 已持久化稳定事件标识，此处只做 JSON 编码和同步发布
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = p.publisher.Publish(ctx, &stream.Message{Payload: payload, Time: event.OccurredAt})
	return err
}

// Run 等待退出并关闭评论事件 Kafka 发布器。
func (p *CommentEventPublisher) Run(ctx context.Context) error {
	// 1. 等待 Leo 生命周期结束后阻止新发布并关闭连接
	<-ctx.Done()
	p.mutex.Lock()
	p.closed = true
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err := p.publisher.Close(cleanupCtx)
	p.mutex.Unlock()
	return err
}

// PublishCommentCountDeadLetter 发布评论计数消费失败消息。
func (p *CommentEventDeadLetterPublisher) PublishCommentCountDeadLetter(ctx context.Context, payload []byte, cause string) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if p.closed {
		return stream.ErrPublisherClosed
	}
	// 1. 保留原始负载并附加失败原因
	message := &stream.Message{Payload: append([]byte(nil), payload...), Header: stream.Header{}}
	message.Header.Set("x-dead-letter-error", cause)
	_, err := p.publisher.Publish(ctx, message)
	return err
}

// Run 等待退出并关闭评论事件死信发布器。
func (p *CommentEventDeadLetterPublisher) Run(ctx context.Context) error {
	// 1. 等待 Leo 生命周期结束后关闭连接
	<-ctx.Done()
	p.mutex.Lock()
	p.closed = true
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err := p.publisher.Close(cleanupCtx)
	p.mutex.Unlock()
	return err
}

// newCommentPublisher 创建指定用途的 Leo Kafka 发布器。
func newCommentPublisher(config *conf.KafkaProducer_Config, name string) (stream.Publisher, error) {
	// 1. 启动时拒绝缺少 Broker 或 Topic 的配置
	if config == nil || config.GetBootstrapServers() == "" || config.GetTopic() == "" {
		return nil, fmt.Errorf("缺少%s Kafka producer 配置", name)
	}
	factory := func() (*confluent.Producer, error) {
		values := confluent.ConfigMap{"bootstrap.servers": config.GetBootstrapServers()}
		for key, value := range config.GetConfigMap() {
			values[key] = value
		}
		return confluent.NewProducer(&values)
	}
	return leokafka.NewPublisher(config.GetTopic(), factory)
}
