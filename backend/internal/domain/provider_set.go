package domain

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/repo"
	"github.com/google/wire"
)

// DomainProviderAppSet is service providers.
var DomainProviderAppSet = wire.NewSet(
	repo.NewHdRepo,
	book.NewHelloworld,
)
