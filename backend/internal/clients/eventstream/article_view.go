// Package eventstream 提供文章浏览事件使用的 Kafka 适配器。
package eventstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/leo/leo/log"
	"codeup.aliyun.com/qimao/leo/leo/stream"
	leokafka "codeup.aliyun.com/qimao/leo/leo/stream/kafka"
	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// ArticleViewPublisher 将文章浏览领域事件发布到 Kafka。
type ArticleViewPublisher struct {
	publisher stream.Publisher     // publisher 是 Leo Kafka 发布器。
	queue     chan *stream.Message // queue 是 HTTP 请求与 Kafka 网络发送之间的有界队列。
	mutex     sync.RWMutex         // mutex 协调请求入队和关闭排空。
	closed    bool                 // closed 表示 Runner 已进入关闭阶段。
}

const (
	articleViewQueueSize       = 256                    // articleViewQueueSize 是进程内浏览事件缓冲数量。
	articleViewPublishAttempts = 3                      // articleViewPublishAttempts 是 Kafka 发送最大尝试次数。
	articleViewPublishDelay    = 100 * time.Millisecond // articleViewPublishDelay 是发送重试初始间隔。
)

// ArticleViewDeadLetterPublisher 将处理失败的浏览消息发布到死信主题。
type ArticleViewDeadLetterPublisher struct {
	publisher stream.Publisher // publisher 是 Leo Kafka 死信发布器。
	mutex     sync.RWMutex     // mutex 协调死信发布和关闭。
	closed    bool             // closed 表示发布器已关闭。
}

// ArticleViewSubscriber 包装文章浏览 Kafka 订阅器以供 Wire 区分类型。
type ArticleViewSubscriber struct {
	stream.Subscriber // Subscriber 提供 Leo Stream 订阅和确认能力。
}

// NewArticleViewPublisher 创建文章浏览事件 Kafka 发布器。
func NewArticleViewPublisher(config *conf.Config) (*ArticleViewPublisher, error) {
	// 1. 读取文章浏览生产者配置并创建 Leo Kafka 发布器
	publisher, err := newPublisher(config.GetData().GetKafka().GetProducer().GetArticleView())
	if err != nil {
		return nil, err
	}
	return &ArticleViewPublisher{publisher: publisher, queue: make(chan *stream.Message, articleViewQueueSize)}, nil
}

// NewArticleViewDeadLetterPublisher 创建文章浏览死信 Kafka 发布器。
func NewArticleViewDeadLetterPublisher(config *conf.Config) (*ArticleViewDeadLetterPublisher, error) {
	// 1. 读取死信主题配置并创建 Leo Kafka 发布器
	publisher, err := newPublisher(config.GetData().GetKafka().GetProducer().GetArticleViewDeadLetter())
	if err != nil {
		return nil, err
	}
	return &ArticleViewDeadLetterPublisher{publisher: publisher}, nil
}

// NewArticleViewSubscriber 创建文章浏览事件 Kafka 订阅器。
func NewArticleViewSubscriber(config *conf.Config) (*ArticleViewSubscriber, error) {
	// 1. 校验文章浏览消费者配置
	cfg := config.GetData().GetKafka().GetConsumer().GetArticleView()
	if cfg == nil || cfg.GetBootstrapServers() == "" || cfg.GetTopic() == "" || cfg.GetGroupId() == "" {
		return nil, fmt.Errorf("缺少文章浏览 Kafka consumer 配置")
	}
	factory := func() (*confluent.Consumer, error) {
		values := confluent.ConfigMap{
			"bootstrap.servers":  cfg.GetBootstrapServers(),
			"group.id":           cfg.GetGroupId(),
			"auto.offset.reset":  "earliest",
			"enable.auto.commit": false,
		}
		for key, value := range cfg.GetConfigMap() {
			values[key] = value
		}
		return confluent.NewConsumer(&values)
	}
	subscriber, err := leokafka.NewSubscriber(cfg.GetTopic(), factory, leokafka.AutoCommit(false))
	if err != nil {
		return nil, err
	}
	return &ArticleViewSubscriber{Subscriber: subscriber}, nil
}

// PublishView 发布一次文章浏览事实。
func (p *ArticleViewPublisher) PublishView(ctx context.Context, event article.ViewEvent) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if p.closed {
		return stream.ErrPublisherClosed
	}
	// 1. 使用 JSON 保持事件契约可观测且跨语言可消费
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	message := &stream.Message{Payload: payload, Time: event.ViewedAt}

	// 2. 请求只写入有界队列，不等待 Kafka 网络回执
	select {
	case p.queue <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("文章浏览事件队列已满")
	}
}

// Run 从有界队列异步发布文章浏览事件。
func (p *ArticleViewPublisher) Run(ctx context.Context) error {
	// 1. 由 Leo Runner 管理队列发送生命周期
	for {
		select {
		case <-ctx.Done():
			return p.shutdown(ctx)
		case message := <-p.queue:
			// 2. 单条消息有限重试后记录错误，不阻断后续浏览事件
			if err := p.publishWithRetry(ctx, message); err != nil {
				log.L().WithContext(ctx).Error("异步发布文章浏览事件失败", err)
			}
		}
	}
}

// shutdown 停止入队、排空已接收消息并关闭 Kafka 发布器。
func (p *ArticleViewPublisher) shutdown(ctx context.Context) error {
	// 1. 与请求入队互斥地标记关闭，确保不会遗漏已接受消息
	p.mutex.Lock()
	p.closed = true
	p.mutex.Unlock()
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	// 2. 排空队列并汇总发送错误
	var shutdownErr error
	for {
		select {
		case message := <-p.queue:
			shutdownErr = errors.Join(shutdownErr, p.publishWithRetry(cleanupCtx, message))
		default:
			return errors.Join(shutdownErr, p.publisher.Close(cleanupCtx))
		}
	}
}

// publishWithRetry 使用指数退避发送单条浏览事件。
func (p *ArticleViewPublisher) publishWithRetry(ctx context.Context, message *stream.Message) error {
	// 1. 首次立即发送，瞬时失败按指数间隔重试
	delay := articleViewPublishDelay
	var err error
	for attempt := 0; attempt < articleViewPublishAttempts; attempt++ {
		if _, err = p.publisher.Publish(ctx, message); err == nil {
			return nil
		}
		if attempt == articleViewPublishAttempts-1 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
		delay *= 2
	}
	return err
}

// PublishDeadLetter 将失败负载投递到死信主题。
func (p *ArticleViewDeadLetterPublisher) PublishDeadLetter(ctx context.Context, payload []byte, cause string) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if p.closed {
		return stream.ErrPublisherClosed
	}
	// 1. 保留原始负载并添加失败原因，便于人工或自动重放
	deadLetter := &stream.Message{Payload: append([]byte(nil), payload...), Header: stream.Header{}}
	deadLetter.Header.Set("x-dead-letter-error", cause)
	_, err := p.publisher.Publish(ctx, deadLetter)
	return err
}

// Run 等待进程退出并返回 Kafka 死信发布器关闭错误。
func (p *ArticleViewDeadLetterPublisher) Run(ctx context.Context) error {
	// 1. 等待 Leo 生命周期发出退出信号
	<-ctx.Done()

	// 2. 等待正在进行的死信发布完成后关闭 Kafka 发布器
	p.mutex.Lock()
	p.closed = true
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err := p.publisher.Close(cleanupCtx)
	p.mutex.Unlock()
	return err
}

// newPublisher 根据 Kafka 生产者配置创建 Leo 发布器。
func newPublisher(config *conf.KafkaProducer_Config) (stream.Publisher, error) {
	// 1. 校验 Kafka 地址和主题
	if config == nil || config.GetBootstrapServers() == "" || config.GetTopic() == "" {
		return nil, fmt.Errorf("缺少文章浏览 Kafka producer 配置")
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
