package job

import (
	"github.com/google/wire"
)

var ProviderJobSet = wire.NewSet(
	NewJob,
)

// ArticleRecoveryProviderSet 提供 HTTP 进程使用的文章对象恢复任务。
var ArticleRecoveryProviderSet = wire.NewSet(
	NewArticleDeletionReconciler,
)
