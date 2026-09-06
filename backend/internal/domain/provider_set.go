package domain

import (
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	articlerepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/repo"
	comment "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	commentrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo"
	like "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	likerepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	userrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/repo"
	"github.com/google/wire"
)

// DomainProviderAppSet 提供现有通用领域服务依赖。
var DomainProviderAppSet = wire.NewSet(
	repo.NewHdRepo,
	book.NewHelloworld,
	ArticleRepositoryProviderSet,
	LikeProviderSet,
	wire.Bind(new(article.Repository), new(*articlerepo.Repository)),
	wire.Bind(new(article.SubmissionGuard), new(*articlerepo.SubmissionGuard)),
	article.NewService,
	wire.Bind(new(article.UseCase), new(*article.Service)),
	wire.Bind(new(article.DeletionRecovery), new(*article.Service)),
	CommentProviderSet,
)

// LikeProviderSet 提供点赞事实、Redis 集合、查询契约和领域服务。
var LikeProviderSet = wire.NewSet(
	likerepo.ProvideTransactionClient,
	likerepo.NewRepository,
	likerepo.NewCache,
	likerepo.NewQueryRepository,
	wire.Bind(new(like.Repository), new(*likerepo.Repository)),
	wire.Bind(new(like.OutboxRepository), new(*likerepo.Repository)),
	wire.Bind(new(like.Cache), new(*likerepo.Cache)),
	wire.Bind(new(article.LikeReader), new(*likerepo.QueryRepository)),
	article.NewPublicationQuery,
	wire.Bind(new(like.ArticleReader), new(*article.PublicationQuery)),
	like.NewService,
	wire.Bind(new(like.UseCase), new(*like.Service)),
	wire.Bind(new(like.CacheRebuilder), new(*like.Service)),
)

// CommentProviderSet 提供评论上下文的仓储、查询适配器和领域服务。
var CommentProviderSet = wire.NewSet(
	articlerepo.NewSubmissionGuard,
	commentrepo.ProvideTransactionClient,
	commentrepo.NewRepository,
	wire.Bind(new(comment.Repository), new(*commentrepo.Repository)),
	wire.Bind(new(comment.OutboxRepository), new(*commentrepo.Repository)),
	commentrepo.NewArticleReader,
	wire.Bind(new(comment.ArticleReader), new(*commentrepo.ArticleReaderAdapter)),
	commentrepo.NewUserReader,
	wire.Bind(new(comment.UserReader), new(*commentrepo.UserReaderAdapter)),
	wire.Bind(new(comment.SubmissionGuard), new(*articlerepo.SubmissionGuard)),
	comment.NewService,
	wire.Bind(new(comment.UseCase), new(*comment.Service)),
)

// ArticleRepositoryProviderSet 提供文章 MySQL 仓储及事务能力。
var ArticleRepositoryProviderSet = wire.NewSet(
	articlerepo.ProvideTransactionClient,
	articlerepo.NewRepository,
	wire.Bind(new(article.ReadingRepository), new(*articlerepo.Repository)),
	wire.Bind(new(article.CommentCountRepository), new(*articlerepo.Repository)),
	wire.Bind(new(article.LikeCountRepository), new(*articlerepo.Repository)),
)

// ArticleCommentCountProviderSet 提供文章评论数投影能力。
var ArticleCommentCountProviderSet = wire.NewSet(
	article.NewCommentCountProjector,
	wire.Bind(new(article.CommentCountProcessor), new(*article.CommentCountProjector)),
)

// ArticleLikeCountProviderSet 提供文章点赞数投影能力。
var ArticleLikeCountProviderSet = wire.NewSet(
	article.NewLikeCountProjector,
	wire.Bind(new(article.LikeCountProcessor), new(*article.LikeCountProjector)),
)

// ArticleReadingProviderSet 提供公开阅读、浏览消费和热榜领域能力。
var ArticleReadingProviderSet = wire.NewSet(
	articlerepo.NewReadingCache,
	wire.Bind(new(article.ViewEventDeduplicator), new(*articlerepo.ReadingCache)),
	wire.Bind(new(article.HotRankStore), new(*articlerepo.ReadingCache)),
	article.NewViewService,
	wire.Bind(new(article.ReadingUseCase), new(*article.ViewService)),
	wire.Bind(new(article.ViewProcessor), new(*article.ViewService)),
	wire.Bind(new(article.HotRankRebuilder), new(*article.ViewService)),
)

// UserRepositoryProviderSet 提供用户上下文共享的 MySQL 仓储 Adapter。
var UserRepositoryProviderSet = wire.NewSet(
	userrepo.ProvideTransactionClient,
	userrepo.NewUserRepository,
	wire.Bind(new(user.Repository), new(*userrepo.UserRepository)),
)

// UserQueryProviderSet 提供开放 gRPC 所需的用户只读查询能力。
var UserQueryProviderSet = wire.NewSet(
	UserRepositoryProviderSet,
	user.NewPBKDF2PasswordHasher,
	user.NewService,
	wire.Bind(new(user.QueryUseCase), new(*user.Service)),
)

// UserGRPCAuthProviderSet 提供开放 gRPC 复用的用户安全 Redis Nonce 能力。
var UserGRPCAuthProviderSet = wire.NewSet(
	userrepo.NewSessionRepository,
	wire.Bind(new(user.GRPCNonceStore), new(*userrepo.SessionRepository)),
)

// UserProviderSet 提供用户上下文的仓储、密码摘要和领域服务。
var UserProviderSet = wire.NewSet(
	UserRepositoryProviderSet,
	wire.Bind(new(user.AuthRepository), new(*userrepo.UserRepository)),
	userrepo.NewSessionRepository,
	wire.Bind(new(user.SessionManager), new(*userrepo.SessionRepository)),
	wire.Bind(new(user.SessionRepository), new(*userrepo.SessionRepository)),
	wire.Bind(new(user.UseCase), new(*user.Service)),
	user.NewPBKDF2PasswordHasher,
	user.NewServiceWithSecurity,
	wire.Bind(new(user.PasswordChangeTokenStore), new(*userrepo.SessionRepository)),
	wire.Bind(new(user.SessionCleanupReconciler), new(*user.Service)),
)
