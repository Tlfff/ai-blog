package job

import (
	"github.com/google/wire"
)

var ProviderJobSet = wire.NewSet(
	NewJob,
)

// ArticleHTTPJobProviderSet 提供 HTTP 进程使用的文章与点赞后台任务。
var ArticleHTTPJobProviderSet = wire.NewSet(
	NewArticleDeletionReconciler,
	NewArticleHotRankJob,
	NewUserSessionCleanupJob,
	NewArticleLikeCacheRebuildJob,
)
