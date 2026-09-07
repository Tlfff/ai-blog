package repo

import (
	"context"
	"fmt"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo/po"
)

// FindNotificationSnapshot 查询文章作者与标题快照。
func (r *Repository) FindNotificationSnapshot(ctx context.Context, id uint64) (*article.NotificationSnapshot, error) {
	// 1. 文章上下文查询自身数据并只返回通知所需字段
	row := new(po.Article)
	found, err := r.client.Context(ctx).ID(id).Cols("id", "author_id", "title").Get(row)
	if err != nil {
		return nil, fmt.Errorf("查询文章通知快照: %w", err)
	}
	if !found {
		return nil, article.ErrArticleNotFound
	}
	return &article.NotificationSnapshot{ID: row.ID, AuthorID: row.AuthorID, Title: row.Title}, nil
}
