package consumer

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/consumer"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/job"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/eventstream"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"context"

	"codeup.aliyun.com/qimao/leo/leo"
	"codeup.aliyun.com/qimao/leo/leo/log"
	"codeup.aliyun.com/qimao/leo/leo/stream"
	"github.com/spf13/cobra"
)

// blogConsumerCmd 表示博客消息消费者命令。
var blogConsumerCmd = &cobra.Command{
	Use:   "blog",
	Short: "blog-consumer",
	Long:  `运行博客消息消费者`,
	Run: func(cmd *cobra.Command, args []string) {
		streamerApp, f, err := newBlogStreamerApp()
		if err != nil {
			panic(err)
		}
		defer f()
		app := leo.NewApp(
			leo.Logger(log.L()),
			leo.Runners(streamerApp),
		)
		if err := app.Run(context.Background()); err != nil {
			panic(err)
		}
	},
}

// consumerApplication 聚合文章浏览 Streamer 和 Kafka 发布器生命周期。
type consumerApplication struct {
	streamer          *stream.Streamer                             // streamer 是文章浏览消息处理器。
	viewEvents        *eventstream.ArticleViewPublisher            // viewEvents 是领域服务持有的浏览事件发布器。
	deadLetter        *eventstream.ArticleViewDeadLetterPublisher  // deadLetter 是浏览消费失败死信发布器。
	commentEvents     *eventstream.CommentEventPublisher           // commentEvents 是评论 Outbox Kafka 发布器。
	commentDeadLetter *eventstream.CommentEventDeadLetterPublisher // commentDeadLetter 是评论计数消费死信发布器。
	outboxRelay       *job.CommentOutboxRelay                      // outboxRelay 是评论 Outbox 补偿任务。
}

// Run 通过 Leo 生命周期并发运行消息流和 Kafka 发布器。
func (app *consumerApplication) Run(ctx context.Context) error {
	// 1. 统一管理订阅器、普通发布器和死信发布器的退出
	return leo.MutilRunner(app.streamer, app.viewEvents, app.deadLetter, app.commentEvents, app.commentDeadLetter, app.outboxRelay).Run(ctx)
}

// newBlogStreamer 创建博客消息消费应用。
//
// 参数说明：
//   - cf：Kafka 消费配置，包含文章浏览和评论事件缓冲大小。
//   - viewHandler：文章浏览事件处理器。
//   - commentHandler：文章评论数事件处理器。
//   - viewEvents：文章浏览事件发布器。
//   - deadLetter：文章浏览消费死信发布器。
//   - commentEvents：评论 Outbox 事件发布器。
//   - commentDeadLetter：评论计数消费死信发布器。
//   - outboxRelay：评论 Outbox 补偿任务。
func newBlogStreamer(cf *conf.Data, viewHandler *consumer.ArticleViewConsumer, commentHandler *consumer.CommentCountConsumer, viewEvents *eventstream.ArticleViewPublisher, deadLetter *eventstream.ArticleViewDeadLetterPublisher, commentEvents *eventstream.CommentEventPublisher, commentDeadLetter *eventstream.CommentEventDeadLetterPublisher, outboxRelay *job.CommentOutboxRelay) *consumerApplication {
	// 1. 使用两类消费者的最大缓冲配置创建 Leo Streamer
	articleViewConfig := cf.GetKafka().GetConsumer().GetArticleView()
	commentEventConfig := cf.GetKafka().GetConsumer().GetCommentEvent()
	messageBufferSize := articleViewConfig.GetMessageBufferSize()
	if commentEventConfig.GetMessageBufferSize() > messageBufferSize {
		messageBufferSize = commentEventConfig.GetMessageBufferSize()
	}
	streamer := stream.NewStreamer(
		stream.MessageBufferSize(int(messageBufferSize)),
		stream.Handlers(viewHandler, commentHandler),
		stream.ErrorHandler(func(err error) {
			log.Error("error: ", err)
		}),
	)
	return &consumerApplication{streamer: streamer, viewEvents: viewEvents, deadLetter: deadLetter, commentEvents: commentEvents, commentDeadLetter: commentDeadLetter, outboxRelay: outboxRelay}
}

// newArticleViewConsumer 组装文章浏览领域处理器、订阅器和死信发布器。
func newArticleViewConsumer(processor article.ViewProcessor, subscriber *eventstream.ArticleViewSubscriber, deadLetter article.ViewDeadLetterPublisher) *consumer.ArticleViewConsumer {
	// 1. 将基础设施适配器作为 Leo Stream 接缝注入应用消费者
	return consumer.NewArticleViewConsumer(subscriber, processor, deadLetter)
}

// newCommentCountConsumer 组装文章评论数投影消费者。
func newCommentCountConsumer(processor article.CommentCountProcessor, subscriber *eventstream.CommentEventSubscriber, deadLetter article.CommentCountDeadLetterPublisher) *consumer.CommentCountConsumer {
	// 1. 将评论事件基础设施适配器注入文章投影消费者
	return consumer.NewCommentCountConsumer(subscriber, processor, deadLetter)
}

// init 注册博客消息消费者子命令。
func init() {
	// 1. 将文章浏览消费者命令加入根 Consumer 命令
	ConsumerCmd.AddCommand(blogConsumerCmd)
}
