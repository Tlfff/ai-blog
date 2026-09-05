//go:build wireinject
// +build wireinject

package consumer

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/eventstream"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	conf.ProviderSet,
	clients.NewMysqlClient,
	clients.NewRedisClient,
	eventstream.NewArticleViewPublisher,
	eventstream.NewArticleViewDeadLetterPublisher,
	eventstream.NewArticleViewSubscriber,
	wire.Bind(new(article.ViewEventPublisher), new(*eventstream.ArticleViewPublisher)),
	wire.Bind(new(article.ViewDeadLetterPublisher), new(*eventstream.ArticleViewDeadLetterPublisher)),
	domain.ArticleRepositoryProviderSet,
	domain.ArticleReadingProviderSet,
	newArticleViewConsumer,
	newBlogStreamer,
)

func newBlogStreamerApp() (*consumerApplication, func(), error) {
	panic(wire.Build(
		ProviderSet,
	))
}
