package comment

import "context"

// ArticleProjectionFact 是评论上下文向对账任务发布的文章评论计数事实快照。
type ArticleProjectionFact struct {
	CommentID uint64 // CommentID 是评论聚合标识。
	ArticleID uint64 // ArticleID 是所属文章标识。
	Version   int64  // Version 是评论聚合最后事件版本。
	Active    bool   // Active 表示评论当前是否应计入文章评论数。
}

// ArticleProjectionFactReader 定义评论上下文对外提供的只读对账契约。
type ArticleProjectionFactReader interface {
	// ListArticleProjectionFacts 返回全部评论对文章评论数的权威贡献状态。
	ListArticleProjectionFacts(context.Context) ([]ArticleProjectionFact, error)
}
