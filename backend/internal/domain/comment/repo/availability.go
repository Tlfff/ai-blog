package repo

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo/po"
)

// IsActive 查询评论是否存在且未删除。
func (r *Repository) IsActive(ctx context.Context, commentID uint64) (bool, error) {
	// 1. 评论上下文只查询自身状态字段，不向调用方泄漏实体
	count, err := r.client.Context(ctx).Where("id = ? AND status = ?", commentID, comment.StatusNormal).Count(new(po.Comment))
	return count > 0, err
}
