package eventstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/stream"
	leokafka "codeup.aliyun.com/qimao/leo/leo/stream/kafka"
	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

var errMissingArticleLikeNotificationConsumerConfig = errors.New("缺少文章点赞通知 Kafka consumer 配置")

// LikeEventPublisher 同步发布点赞 Outbox 事件。
type LikeEventPublisher struct {
	publisher stream.Publisher // publisher 是 Leo Kafka 发布器。
	mutex     sync.RWMutex     // mutex 协调发布与关闭。
	closed    bool             // closed 表示发布器已关闭。
}

// LikeEventSubscriber 包装点赞事件 Kafka 订阅器。
type LikeEventSubscriber struct {
	stream.Subscriber // Subscriber 提供 Leo Stream 订阅能力。
}

// LikeEventDeadLetterPublisher 发布点赞计数消费死信。
type LikeEventDeadLetterPublisher struct {
	publisher stream.Publisher // publisher 是 Leo Kafka 死信发布器。
	mutex     sync.RWMutex     // mutex 协调发布与关闭。
	closed    bool             // closed 表示发布器已关闭。
}

// NewLikeEventPublisher 创建点赞事件 Kafka 发布器。
func NewLikeEventPublisher(config *conf.Config) (*LikeEventPublisher, error) {
	// 1. 从版本化配置字段创建同步发布器
	publisher, err := newIntegrationPublisher(config.GetData().GetKafka().GetProducer().GetLikeEvent(), "点赞事件")
	if err != nil {
		return nil, err
	}
	return &LikeEventPublisher{publisher: publisher}, nil
}

// NewLikeEventSubscriber 创建点赞计数 Kafka 订阅器。
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

// Publish 同步等待 Kafka 接受点赞事件。
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

// Run 等待退出并关闭点赞事件 Kafka 发布器。
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
	// 1. 复用点赞事件死信 Topic 保留原始负载和失败原因
	return p.publishDeadLetter(ctx, payload, cause)
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

// ArticleLikeNotificationSubscriber 包装文章点赞通知独立消费组订阅器。
type ArticleLikeNotificationSubscriber struct {
	stream.Subscriber // Subscriber 提供通知独立消费组的 Leo Stream 订阅能力。
}

// NewArticleLikeNotificationSubscriber 创建文章点赞通知 Kafka 订阅器。
func NewArticleLikeNotificationSubscriber(config *conf.Config) (*ArticleLikeNotificationSubscriber, error) {
	// 1. 通知使用独立消费组读取与点赞计数相同的 Topic
	cfg := config.GetData().GetKafka().GetConsumer().GetArticleLikeNotification()
	if cfg.GetBootstrapServers() == "" || cfg.GetTopic() == "" || cfg.GetGroupId() == "" {
		return nil, errMissingArticleLikeNotificationConsumerConfig
	}
	// 2. 创建关闭自动提交的 Kafka Consumer，由 Leo 控制确认
	factory := func() (*confluent.Consumer, error) {
		values := confluent.ConfigMap{"bootstrap.servers": cfg.GetBootstrapServers(), "group.id": cfg.GetGroupId(), "auto.offset.reset": "earliest", "enable.auto.commit": false}
		for key, value := range cfg.GetConfigMap() {
			values[key] = value
		}
		return confluent.NewConsumer(&values)
	}
	subscriber, err := leokafka.NewSubscriber(cfg.GetTopic(), factory, leokafka.AutoCommit(false))
	if err != nil {
		return nil, fmt.Errorf("创建文章点赞通知 Kafka 订阅器: %w", err)
	}
	return &ArticleLikeNotificationSubscriber{Subscriber: subscriber}, nil
}

// PublishNotificationDeadLetter 发布通知消费失败的原始点赞消息。
func (p *LikeEventDeadLetterPublisher) PublishNotificationDeadLetter(ctx context.Context, payload []byte, cause string) error {
	// 1. 复用点赞死信 Topic 保存通知消费失败消息
	return p.publishDeadLetter(ctx, payload, cause)
}

// publishDeadLetter 在关闭互斥边界内发布原始负载和失败原因。
func (p *LikeEventDeadLetterPublisher) publishDeadLetter(ctx context.Context, payload []byte, cause string) error {
	// 1. 阻止发布器关闭后继续发送消息
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if p.closed {
		return stream.ErrPublisherClosed
	}
	// 2. 不修改原始事件，仅在 Header 附加失败原因
	message := &stream.Message{Payload: append([]byte(nil), payload...), Header: stream.Header{}}
	message.Header.Set("x-dead-letter-error", cause)
	_, err := p.publisher.Publish(ctx, message)
	if err != nil {
		return fmt.Errorf("发布点赞事件死信: %w", err)
	}
	return nil
}
