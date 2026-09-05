package domain

import (
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	articlerepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/repo"
	comment "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	commentrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo"
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
	likerepo.NewQueryRepository,
	wire.Bind(new(article.LikeReader), new(*likerepo.QueryRepository)),
	wire.Bind(new(article.Repository), new(*articlerepo.Repository)),
	wire.Bind(new(article.SubmissionGuard), new(*articlerepo.SubmissionGuard)),
	article.NewService,
	wire.Bind(new(article.UseCase), new(*article.Service)),
	wire.Bind(new(article.DeletionRecovery), new(*article.Service)),
	CommentProviderSet,
)

// CommentProviderSet 提供评论上下文的仓储、查询适配器和领域服务。
var CommentProviderSet = wire.NewSet(
	articlerepo.NewSubmissionGuard,
	commentrepo.ProvideTransactionClient,
	commentrepo.NewRepository,
	wire.Bind(new(comment.Repository), new(*commentrepo.Repository)),
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

// UserProviderSet 提供用户上下文的仓储、密码摘要和领域服务。
var UserProviderSet = wire.NewSet(
	userrepo.ProvideTransactionClient,
	userrepo.NewUserRepository,
	wire.Bind(new(user.Repository), new(*userrepo.UserRepository)),
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
