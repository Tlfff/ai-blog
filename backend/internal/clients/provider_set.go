package clients

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/ipregion"
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
	wire.Bind(new(user.IPRegionResolver), new(*ipregion.Resolver)),
)
