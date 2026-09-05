package clients

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/eventstream"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/ipregion"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/objectstorage"
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"github.com/google/wire"
)

// ProviderClientsSet is biz providers.
var ProviderClientsSet = wire.NewSet(
	// mysql
	NewMysqlClient,
	NewLogMysqlClient,
	// redis
	NewRedisClient,
	ipregion.NewConfiguredResolver,
	objectstorage.NewStorage,
	objectstorage.ProvideAllowedImageExtensions,
	objectstorage.ProvideAllowedAvatarExtensions,
	wire.Bind(new(user.AvatarStorage), new(*objectstorage.Storage)),
	eventstream.NewArticleViewPublisher,
	wire.Bind(new(article.ViewEventPublisher), new(*eventstream.ArticleViewPublisher)),
	wire.Bind(new(article.Storage), new(*objectstorage.Storage)),
	wire.Bind(new(user.IPRegionResolver), new(*ipregion.Resolver)),
)
