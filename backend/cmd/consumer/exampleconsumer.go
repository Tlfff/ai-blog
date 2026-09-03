package consumer

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/consumer"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
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
func newBlogStreamer(cf *conf.Data, handler *consumer.BlogConsumer) *stream.Streamer {
	streamer := stream.NewStreamer(
		stream.MessageBufferSize(int(cf.GetKafka().GetConsumer().GetBidResultReport().GetMessageBufferSize())),
		stream.Handlers(handler),
		stream.ErrorHandler(func(err error) {
			log.Error("error: ", err)
		}),
	)
	return streamer
}

func init() {
	ConsumerCmd.AddCommand(blogConsumerCmd)
}
