package domain

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	userrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/repo"
	"github.com/google/wire"
)

// DomainProviderAppSet 提供现有通用领域服务依赖。
var DomainProviderAppSet = wire.NewSet(
	repo.NewHdRepo,
	book.NewHelloworld,
)

// UserProviderSet 提供用户上下文的仓储、密码摘要和领域服务。
var UserProviderSet = wire.NewSet(
	userrepo.NewUserRepository,
	userrepo.NewSessionRepository,
	user.NewPBKDF2PasswordHasher,
	user.NewService,
)
