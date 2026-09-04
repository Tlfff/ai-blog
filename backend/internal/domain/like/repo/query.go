// Package repo 提供点赞上下文面向其他上下文的稳定查询适配器。
package repo

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
)

// QueryRepository 使用点赞关系事实表实现只读查询契约。
type QueryRepository struct {
	client clients.MysqlClient // client 提供点赞关系只读查询能力。
}

// NewQueryRepository 创建点赞关系查询仓储。
func NewQueryRepository(client clients.MysqlClient) *QueryRepository {
	// 1. 启动阶段拒绝缺少 MySQL 客户端
	if client == nil {
		panic("点赞查询仓储缺少 MySQL 客户端")
	}
	return &QueryRepository{client: client}
}

// IsArticleLiked 查询用户是否存在有效文章点赞关系。
func (r *QueryRepository) IsArticleLiked(ctx context.Context, userID, articleID uint64) (bool, error) {
	// 1. 未登录用户固定视为未点赞
	if userID == 0 {
		return false, nil
	}
	// 2. 点赞上下文只暴露布尔查询结果，不泄漏关系实体
	count, err := r.client.Context(ctx).
		Table("article_likes").
		Where("user_id = ? AND article_id = ? AND status = ?", userID, articleID, 1).
		Count()
	return count > 0, err
}
