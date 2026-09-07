//go:build wireinject
// +build wireinject

package consumer

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/job"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/eventstream"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	commentrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	likerepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	conf.ProviderSet,
	clients.NewMysqlClient,
	clients.NewRedisClient,
	clients.NewMongoClient,
	eventstream.NewArticleViewPublisher,
	eventstream.NewArticleViewDeadLetterPublisher,
	eventstream.NewArticleViewSubscriber,
	eventstream.NewCommentEventPublisher,
	eventstream.NewCommentEventDeadLetterPublisher,
	eventstream.NewCommentEventSubscriber,
	eventstream.NewLikeEventPublisher,
	eventstream.NewLikeEventDeadLetterPublisher,
	eventstream.NewLikeEventSubscriber,
	eventstream.NewArticleLikeNotificationSubscriber,
	wire.Bind(new(article.ViewEventPublisher), new(*eventstream.ArticleViewPublisher)),
	wire.Bind(new(article.ViewDeadLetterPublisher), new(*eventstream.ArticleViewDeadLetterPublisher)),
	wire.Bind(new(comment.EventPublisher), new(*eventstream.CommentEventPublisher)),
	wire.Bind(new(article.CommentCountDeadLetterPublisher), new(*eventstream.CommentEventDeadLetterPublisher)),
	wire.Bind(new(like.EventPublisher), new(*eventstream.LikeEventPublisher)),
	wire.Bind(new(article.LikeCountDeadLetterPublisher), new(*eventstream.LikeEventDeadLetterPublisher)),
	wire.Bind(new(notification.DeadLetterPublisher), new(*eventstream.LikeEventDeadLetterPublisher)),
	domain.ArticleRepositoryProviderSet,
	domain.ArticleReadingProviderSet,
	domain.ArticleCommentCountProviderSet,
	domain.ArticleLikeCountProviderSet,
	domain.CommentLikeCountProviderSet,
	domain.UserQueryProviderSet,
	domain.NotificationProviderSet,
	commentrepo.ProvideTransactionClient,
	commentrepo.NewRepository,
	wire.Bind(new(comment.OutboxRepository), new(*commentrepo.Repository)),
	wire.Bind(new(comment.LikeCountRepository), new(*commentrepo.Repository)),
	likerepo.ProvideTransactionClient,
	likerepo.NewRepository,
	wire.Bind(new(like.OutboxRepository), new(*likerepo.Repository)),
	wire.Bind(new(like.CommentOutboxRepository), new(*likerepo.Repository)),
	job.NewCommentOutboxRelay,
	job.NewLikeOutboxRelay,
	job.NewCommentLikeOutboxRelay,
	newArticleViewConsumer,
	newCommentCountConsumer,
	newLikeCountConsumer,
	newNotificationConsumer,
	newBlogStreamer,
)

// newBlogStreamerApp 组装博客消息消费进程及资源清理函数。
func newBlogStreamerApp() (*consumerApplication, func(), error) {
	// 1. 由 Wire 生成实际依赖组装实现
	panic(wire.Build(
		ProviderSet,
	))
}
