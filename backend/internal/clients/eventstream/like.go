package eventstream

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/stream"
	leokafka "codeup.aliyun.com/qimao/leo/leo/stream/kafka"
	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// LikeEventPublisher 同步发布文章点赞 Outbox 事件。
type LikeEventPublisher struct {
	publisher stream.Publisher // publisher 是 Leo Kafka 发布器。
	mutex     sync.RWMutex     // mutex 协调发布与关闭。
	closed    bool             // closed 表示发布器已关闭。
}

// LikeEventSubscriber 包装文章点赞事件 Kafka 订阅器。
type LikeEventSubscriber struct {
	stream.Subscriber // Subscriber 提供 Leo Stream 订阅能力。
}

// LikeEventDeadLetterPublisher 发布文章点赞计数消费死信。
type LikeEventDeadLetterPublisher struct {
	publisher stream.Publisher // publisher 是 Leo Kafka 死信发布器。
	mutex     sync.RWMutex     // mutex 协调发布与关闭。
	closed    bool             // closed 表示发布器已关闭。
}

// NewLikeEventPublisher 创建文章点赞事件 Kafka 发布器。
func NewLikeEventPublisher(config *conf.Config) (*LikeEventPublisher, error) {
	// 1. 从版本化配置字段创建同步发布器
	publisher, err := newIntegrationPublisher(config.GetData().GetKafka().GetProducer().GetLikeEvent(), "点赞事件")
	if err != nil {
		return nil, err
	}
	return &LikeEventPublisher{publisher: publisher}, nil
}

// NewLikeEventSubscriber 创建文章点赞计数 Kafka 订阅器。
func NewLikeEventSubscriber(config *conf.Config) (*LikeEventSubscriber, error) {
	// 1. 从版本化配置字段创建手动提交订阅器
	cfg := config.GetData().GetKafka().GetConsumer().GetLikeEvent()
	if cfg.GetBootstrapServers() == "" || cfg.GetTopic() == "" || cfg.GetGroupId() == "" {
		return nil, fmt.Errorf("缺少点赞事件 Kafka consumer 配置")
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
	return &LikeEventSubscriber{Subscriber: subscriber}, nil
}

// NewLikeEventDeadLetterPublisher 创建点赞计数死信发布器。
func NewLikeEventDeadLetterPublisher(config *conf.Config) (*LikeEventDeadLetterPublisher, error) {
	// 1. 从版本化配置字段创建死信发布器
	publisher, err := newIntegrationPublisher(config.GetData().GetKafka().GetProducer().GetLikeEventDeadLetter(), "点赞事件死信")
	if err != nil {
		return nil, err
	}
	return &LikeEventDeadLetterPublisher{publisher: publisher}, nil
}

// Publish 同步等待 Kafka 接受文章点赞事件。
func (p *LikeEventPublisher) Publish(ctx context.Context, event like.IntegrationEvent) error {
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

// Run 等待退出并关闭文章点赞事件 Kafka 发布器。
func (p *LikeEventPublisher) Run(ctx context.Context) error {
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

// PublishLikeCountDeadLetter 发布点赞计数消费失败消息。
func (p *LikeEventDeadLetterPublisher) PublishLikeCountDeadLetter(ctx context.Context, payload []byte, cause string) error {
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

// Run 等待退出并关闭点赞事件死信发布器。
func (p *LikeEventDeadLetterPublisher) Run(ctx context.Context) error {
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
