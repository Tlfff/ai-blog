// Package repo 提供点赞上下文的 MySQL 事实仓储和 Redis 查询适配器。
package repo

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
)

// QueryRepository 使用 Redis 正缓存和 MySQL 事实表实现只读查询契约。
type QueryRepository struct {
	client clients.MysqlClient // client 提供点赞关系权威查询能力。
	cache  *Cache              // cache 提供可丢失的 Redis 正缓存。
}

// NewQueryRepository 创建点赞关系查询仓储。
func NewQueryRepository(client clients.MysqlClient, cache *Cache) *QueryRepository {
	// 1. 启动阶段拒绝缺少 MySQL 或 Redis 适配器
	if client == nil || cache == nil {
		panic("点赞查询仓储缺少必要依赖")
	}
	return &QueryRepository{client: client, cache: cache}
}

// IsArticleLiked 查询用户是否存在有效文章点赞关系。
func (r *QueryRepository) IsArticleLiked(ctx context.Context, userID, articleID uint64) (bool, error) {
	// 1. 未登录用户固定视为未点赞
	if userID == 0 {
		return false, nil
	}

	// 2. 始终以 MySQL 点赞事实回答，避免 Redis 更新失败造成陈旧正缓存
	count, err := r.client.Context(ctx).
		Table("article_likes").
		Where("user_id = ? AND article_id = ? AND status = ?", userID, articleID, like.StatusLiked).
		Count()
	if err != nil {
		return false, err
	}
	liked := count > 0

	// 3. 使用查询到的最终事实尽力修复 Redis，缓存故障不影响结果
	_ = r.cache.StoreArticleLike(ctx, articleID, userID, liked)
	return liked, nil
}
