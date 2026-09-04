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

// newBlogStreamer 创建博客消息流运行器。
func newBlogStreamer(cf *conf.Data, handler *consumer.ArticleViewConsumer) *stream.Streamer {
	articleViewConfig := cf.GetKafka().GetConsumer().GetArticleView()
	streamer := stream.NewStreamer(
		stream.MessageBufferSize(int(articleViewConfig.GetMessageBufferSize())),
		stream.Handlers(handler),
		stream.ErrorHandler(func(err error) {
			log.Error("error: ", err)
		}),
	)
	return streamer
}

// newArticleViewConsumer 组装文章浏览领域处理器、订阅器和死信发布器。
func newArticleViewConsumer(processor article.ViewProcessor, subscriber *eventstream.ArticleViewSubscriber, deadLetter article.ViewDeadLetterPublisher) *consumer.ArticleViewConsumer {
	// 1. 将基础设施适配器作为 Leo Stream 接缝注入应用消费者
	return consumer.NewArticleViewConsumer(subscriber, processor, deadLetter)
}

func init() {
	ConsumerCmd.AddCommand(blogConsumerCmd)
}
