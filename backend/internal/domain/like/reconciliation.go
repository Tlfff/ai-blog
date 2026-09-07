package like

import "context"

// ArticleProjectionFact 是点赞上下文向对账任务发布的文章点赞计数事实快照。
type ArticleProjectionFact struct {
	LikeID    uint64 // LikeID 是点赞关系标识。
	ArticleID uint64 // ArticleID 是目标文章标识。
	UserID    uint64 // UserID 是点赞用户标识。
	Version   int64  // Version 是点赞关系最后事件版本。
	Active    bool   // Active 表示关系当前是否应计入文章点赞数。
}

// ArticleProjectionFactReader 定义点赞上下文对外提供的只读对账契约。
type ArticleProjectionFactReader interface {
	// ListArticleProjectionFacts 返回全部文章点赞关系的权威贡献状态。
	ListArticleProjectionFacts(context.Context) ([]ArticleProjectionFact, error)
}
