package repo

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo/po"
)

// IsPublished 查询文章是否存在且处于已发表状态。
func (r *Repository) IsPublished(ctx context.Context, articleID uint64) (bool, error) {
	// 1. 文章上下文只查询自身状态字段，不向调用方泄漏实体
	count, err := r.client.Context(ctx).Where("id = ? AND status = ?", articleID, article.StatusPublished).Count(new(po.Article))
	return count > 0, err
}
