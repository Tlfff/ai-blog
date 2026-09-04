package domain

import (
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	articlerepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/repo"
	likerepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	userrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/repo"
	"github.com/google/wire"
)

// DomainProviderAppSet 提供现有通用领域服务依赖。
var DomainProviderAppSet = wire.NewSet(
	repo.NewHdRepo,
	book.NewHelloworld,
	articlerepo.ProvideTransactionClient,
	articlerepo.NewRepository,
	articlerepo.NewSubmissionGuard,
	likerepo.NewQueryRepository,
	wire.Bind(new(article.LikeReader), new(*likerepo.QueryRepository)),
	wire.Bind(new(article.Repository), new(*articlerepo.Repository)),
	wire.Bind(new(article.SubmissionGuard), new(*articlerepo.SubmissionGuard)),
	article.NewService,
	wire.Bind(new(article.UseCase), new(*article.Service)),
	wire.Bind(new(article.DeletionRecovery), new(*article.Service)),
)

// UserProviderSet 提供用户上下文的仓储、密码摘要和领域服务。
var UserProviderSet = wire.NewSet(
	userrepo.NewUserRepository,
	wire.Bind(new(user.Repository), new(*userrepo.UserRepository)),
	wire.Bind(new(user.AuthRepository), new(*userrepo.UserRepository)),
	userrepo.NewSessionRepository,
	wire.Bind(new(user.SessionManager), new(*userrepo.SessionRepository)),
	wire.Bind(new(user.SessionRepository), new(*userrepo.SessionRepository)),
	user.NewPBKDF2PasswordHasher,
	user.NewServiceWithSession,
)
