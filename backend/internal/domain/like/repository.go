package like

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/entity"
)

const (
	StatusLiked   int8 = 1 // StatusLiked 表示用户当前已点赞文章。
	StatusUnliked int8 = 2 // StatusUnliked 表示用户当前未点赞文章。
)

// Repository 定义点赞事实和事务 Outbox 所需的数据能力。
type Repository interface {
	// ChangeArticleLike 幂等变更点赞事实，并为实际状态变化原子写入 Outbox。
	ChangeArticleLike(context.Context, uint64, uint64, int8, time.Time) (bool, error)
	// ListActiveArticleLikes 查询全部当前生效的文章点赞事实，用于重建 Redis。
	ListActiveArticleLikes(context.Context) ([]*entity.ArticleLike, error)
	// ChangeCommentLike 幂等变更评论点赞事实，并为实际状态变化原子写入 Outbox。
	ChangeCommentLike(context.Context, uint64, uint64, int8, time.Time) (bool, error)
	// ListActiveCommentLikes 查询全部当前生效的评论点赞事实，用于重建 Redis。
	ListActiveCommentLikes(context.Context) ([]*entity.CommentLike, error)
}

// ArticleReader 定义点赞上下文校验公开文章的稳定查询契约。
type ArticleReader interface {
	// IsPublished 查询文章是否存在且处于已发表状态。
	IsPublished(context.Context, uint64) (bool, error)
}

// CommentReader 定义点赞上下文校验正常评论的稳定查询契约。
type CommentReader interface {
	// IsActive 查询评论是否存在且未删除。
	IsActive(context.Context, uint64) (bool, error)
}

// Cache 定义文章和评论点赞 Redis 集合的可丢失投影能力。
type Cache interface {
	// IsArticleLiked 查询 Redis 集合中是否存在点赞用户。
	IsArticleLiked(context.Context, uint64, uint64) (bool, error)
	// StoreArticleLike 使用最终事实覆盖单个用户的缓存状态。
	StoreArticleLike(context.Context, uint64, uint64, bool) error
	// ReplaceArticleLikes 使用 MySQL 当前事实重建全部文章点赞集合。
	ReplaceArticleLikes(context.Context, []*entity.ArticleLike) error
	// StoreCommentLike 使用最终事实覆盖单个用户的评论点赞缓存状态。
	StoreCommentLike(context.Context, uint64, uint64, bool) error
	// ReplaceCommentLikes 使用 MySQL 当前事实重建全部评论点赞集合。
	ReplaceCommentLikes(context.Context, []*entity.CommentLike) error
}

// CacheRebuilder 定义从 MySQL 事实恢复 Redis 点赞集合的能力。
type CacheRebuilder interface {
	// RebuildArticleLikeCache 重建全部文章点赞集合。
	RebuildArticleLikeCache(context.Context) error
}

// CommentCacheRebuilder 定义从 MySQL 事实恢复 Redis 评论点赞集合的能力。
type CommentCacheRebuilder interface {
	// RebuildCommentLikeCache 重建全部评论点赞集合。
	RebuildCommentLikeCache(context.Context) error
}
