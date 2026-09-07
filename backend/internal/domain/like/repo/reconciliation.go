package repo

import (
	"context"
	"fmt"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
)

// ListArticleProjectionFacts 发布点赞上下文拥有的完整文章点赞关系快照。
func (r *Repository) ListArticleProjectionFacts(ctx context.Context) ([]like.ArticleProjectionFact, error) {
	rows := make([]struct {
		LikeID    uint64 `xorm:"'like_id'"`
		ArticleID uint64 `xorm:"'article_id'"`
		UserID    uint64 `xorm:"'user_id'"`
		Status    int8   `xorm:"'status'"`
		Version   int64  `xorm:"'version'"`
	}, 0)
	query := `SELECT l.id AS like_id, l.article_id, l.user_id, l.status,
COALESCE(MAX(o.version), CASE WHEN l.status = 1 THEN 1 ELSE 2 END) AS version
FROM article_likes l
LEFT JOIN article_like_event_outbox o ON o.aggregate_id = l.id
GROUP BY l.id, l.article_id, l.user_id, l.status
ORDER BY l.id`
	if err := r.client.Context(ctx).SQL(query).Find(&rows); err != nil {
		return nil, fmt.Errorf("读取文章点赞投影事实: %w", err)
	}
	facts := make([]like.ArticleProjectionFact, 0, len(rows))
	for _, row := range rows {
		facts = append(facts, like.ArticleProjectionFact{
			LikeID:    row.LikeID,
			ArticleID: row.ArticleID,
			UserID:    row.UserID,
			Version:   row.Version,
			Active:    row.Status == like.StatusLiked,
		})
	}
	return facts, nil
}
