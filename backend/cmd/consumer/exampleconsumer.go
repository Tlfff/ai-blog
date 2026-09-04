package consumer

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/consumer"
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
	streamer   *stream.Streamer                            // streamer 是文章浏览消息处理器。
	viewEvents *eventstream.ArticleViewPublisher           // viewEvents 是领域服务持有的浏览事件发布器。
	deadLetter *eventstream.ArticleViewDeadLetterPublisher // deadLetter 是消费失败死信发布器。
}

// Run 通过 Leo 生命周期并发运行消息流和 Kafka 发布器。
func (app *consumerApplication) Run(ctx context.Context) error {
	// 1. 统一管理订阅器、普通发布器和死信发布器的退出
	return leo.MutilRunner(app.streamer, app.viewEvents, app.deadLetter).Run(ctx)
}

// newBlogStreamer 创建博客消息消费应用。
func newBlogStreamer(cf *conf.Data, handler *consumer.ArticleViewConsumer, viewEvents *eventstream.ArticleViewPublisher, deadLetter *eventstream.ArticleViewDeadLetterPublisher) *consumerApplication {
	// 1. 使用文章浏览消费者配置创建 Leo Streamer
	articleViewConfig := cf.GetKafka().GetConsumer().GetArticleView()
	streamer := stream.NewStreamer(
		stream.MessageBufferSize(int(articleViewConfig.GetMessageBufferSize())),
		stream.Handlers(handler),
		stream.ErrorHandler(func(err error) {
			log.Error("error: ", err)
		}),
	)
	return &consumerApplication{streamer: streamer, viewEvents: viewEvents, deadLetter: deadLetter}
}

// newArticleViewConsumer 组装文章浏览领域处理器、订阅器和死信发布器。
func newArticleViewConsumer(processor article.ViewProcessor, subscriber *eventstream.ArticleViewSubscriber, deadLetter article.ViewDeadLetterPublisher) *consumer.ArticleViewConsumer {
	// 1. 将基础设施适配器作为 Leo Stream 接缝注入应用消费者
	return consumer.NewArticleViewConsumer(subscriber, processor, deadLetter)
}

// init 注册博客消息消费者子命令。
func init() {
	// 1. 将文章浏览消费者命令加入根 Consumer 命令
	ConsumerCmd.AddCommand(blogConsumerCmd)
}
