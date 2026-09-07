package repo

import (
	"context"
	"fmt"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
)

// ListArticleProjectionFacts 发布评论上下文拥有的完整评论状态快照。
func (r *Repository) ListArticleProjectionFacts(ctx context.Context) ([]comment.ArticleProjectionFact, error) {
	rows := make([]struct {
		CommentID uint64 `xorm:"'comment_id'"`
		ArticleID uint64 `xorm:"'article_id'"`
		Status    int8   `xorm:"'status'"`
		Version   int64  `xorm:"'version'"`
	}, 0)
	query := `SELECT c.id AS comment_id, c.article_id, c.status,
COALESCE(MAX(o.version), CASE WHEN c.status = 1 THEN 1 ELSE 2 END) AS version
FROM comments c
LEFT JOIN comment_event_outbox o ON o.aggregate_id = c.id
GROUP BY c.id, c.article_id, c.status
ORDER BY c.id`
	if err := r.client.Context(ctx).SQL(query).Find(&rows); err != nil {
		return nil, fmt.Errorf("读取评论投影事实: %w", err)
	}
	facts := make([]comment.ArticleProjectionFact, 0, len(rows))
	for _, row := range rows {
		facts = append(facts, comment.ArticleProjectionFact{
			CommentID: row.CommentID,
			ArticleID: row.ArticleID,
			Version:   row.Version,
			Active:    row.Status == comment.StatusNormal,
		})
	}
	return facts, nil
}
