package repo

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo/po"
	"xorm.io/xorm"         //nolint:depguard // 仓储 Adapter 复用项目既有 XORM 事务客户端。
	"xorm.io/xorm/schemas" //nolint:depguard // 方言判断用于兼容 SQLite 仓储测试。
)

// RepairInteractionProjections 合并跨上下文完整快照并重算文章互动计数。
//
//nolint:wrapcheck // XORM 错误在上层领域服务统一补充文章互动投影语义。
func (r *Repository) RepairInteractionProjections(ctx context.Context, snapshot article.InteractionProjectionSnapshot) error {
	// 1. 在文章本地事务中串行化关系投影合并和聚合计数重算
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		articleIDs, err := lockAllArticleIDs(session)
		if err != nil {
			return nil, err
		}
		if err := mergeCommentProjectionFacts(session, snapshot, articleIDs); err != nil {
			return nil, err
		}
		if err := mergeLikeProjectionFacts(session, snapshot, articleIDs); err != nil {
			return nil, err
		}
		if err := recalculateInteractionCounts(session); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

// lockAllArticleIDs 锁定当前全部文章并返回有效目标集合。
//
//nolint:wrapcheck // XORM 错误在上层领域服务统一补充文章互动投影语义。
func lockAllArticleIDs(session *xorm.Session) (map[uint64]struct{}, error) {
	// 1. MySQL 使用行锁阻止事件消费者与全量重算交错写入
	rows := make([]struct {
		ID uint64 `xorm:"'id'"`
	}, 0)
	query := "SELECT id FROM articles ORDER BY id"
	if session.Engine().Dialect().URI().DBType == schemas.MYSQL {
		query += " FOR UPDATE"
	}
	if err := session.SQL(query).Find(&rows); err != nil {
		return nil, err
	}
	ids := make(map[uint64]struct{}, len(rows))
	for _, row := range rows {
		ids[row.ID] = struct{}{}
	}
	return ids, nil
}

// mergeCommentProjectionFacts 合并评论权威快照并保留快照开始后的新事件。
//
//nolint:wrapcheck // XORM 错误在上层领域服务统一补充文章互动投影语义。
func mergeCommentProjectionFacts(session *xorm.Session, snapshot article.InteractionProjectionSnapshot, articleIDs map[uint64]struct{}) error {
	// 1. 锁定现有关系投影并构造完整期望集合
	existing := make([]articleCommentProjection, 0)
	if err := forUpdate(session).Find(&existing); err != nil {
		return err
	}
	existingByID := make(map[uint64]articleCommentProjection, len(existing))
	for _, row := range existing {
		existingByID[row.CommentID] = row
	}
	desired := make(map[uint64]article.CommentProjectionFact, len(snapshot.Comments))
	for _, fact := range snapshot.Comments {
		if _, exists := articleIDs[fact.ArticleID]; exists {
			desired[fact.CommentID] = fact
		}
	}
	// 2. 删除事实源已不存在且未被并发新事件更新的陈旧关系
	for _, row := range existing {
		_, wanted := desired[row.CommentID]
		_, articleExists := articleIDs[row.ArticleID]
		if (!wanted || !articleExists) && !row.UpdatedTime.After(snapshot.CapturedAt) {
			if _, err := session.ID(row.CommentID).Delete(new(articleCommentProjection)); err != nil {
				return err
			}
		}
	}
	// 3. 覆盖快照边界前的关系状态，快照边界后的事件结果保持优先
	now := time.Now()
	for id, fact := range desired {
		current, found := existingByID[id]
		if found && current.UpdatedTime.After(snapshot.CapturedAt) {
			continue
		}
		active := int8(0)
		if fact.Active {
			active = 1
		}
		row := &articleCommentProjection{CommentID: fact.CommentID, ArticleID: fact.ArticleID, Version: fact.Version, Active: active, LastEventID: "reconcile", UpdatedTime: now}
		if found {
			if _, err := session.ID(fact.CommentID).AllCols().Update(row); err != nil {
				return err
			}
		} else if _, err := session.Insert(row); err != nil {
			return err
		}
	}
	return nil
}

// mergeLikeProjectionFacts 合并点赞权威快照并保留快照开始后的新事件。
//
//nolint:wrapcheck // XORM 错误在上层领域服务统一补充文章互动投影语义。
func mergeLikeProjectionFacts(session *xorm.Session, snapshot article.InteractionProjectionSnapshot, articleIDs map[uint64]struct{}) error {
	// 1. 锁定现有关系投影并构造完整期望集合
	existing := make([]articleLikeProjection, 0)
	if err := forUpdate(session).Find(&existing); err != nil {
		return err
	}
	existingByID := make(map[uint64]articleLikeProjection, len(existing))
	for _, row := range existing {
		existingByID[row.LikeID] = row
	}
	desired := make(map[uint64]article.LikeProjectionFact, len(snapshot.Likes))
	for _, fact := range snapshot.Likes {
		if _, exists := articleIDs[fact.ArticleID]; exists {
			desired[fact.LikeID] = fact
		}
	}
	// 2. 删除事实源已不存在且未被并发新事件更新的陈旧关系
	for _, row := range existing {
		_, wanted := desired[row.LikeID]
		_, articleExists := articleIDs[row.ArticleID]
		if (!wanted || !articleExists) && !row.UpdatedTime.After(snapshot.CapturedAt) {
			if _, err := session.ID(row.LikeID).Delete(new(articleLikeProjection)); err != nil {
				return err
			}
		}
	}
	// 3. 覆盖快照边界前的关系状态，快照边界后的事件结果保持优先
	now := time.Now()
	for id, fact := range desired {
		current, found := existingByID[id]
		if found && current.UpdatedTime.After(snapshot.CapturedAt) {
			continue
		}
		active := int8(0)
		if fact.Active {
			active = 1
		}
		row := &articleLikeProjection{LikeID: fact.LikeID, ArticleID: fact.ArticleID, UserID: fact.UserID, Version: fact.Version, Active: active, LastEventID: "reconcile", UpdatedTime: now}
		if found {
			if _, err := session.ID(fact.LikeID).AllCols().Update(row); err != nil {
				return err
			}
		} else if _, err := session.Insert(row); err != nil {
			return err
		}
	}
	return nil
}

// recalculateInteractionCounts 从文章上下文关系投影重算全部互动计数。
//
//nolint:wrapcheck // XORM 错误在上层领域服务统一补充文章互动投影语义。
func recalculateInteractionCounts(session *xorm.Session) error {
	// 1. 先清零全部文章，保证无有效关系的文章也能修复漂移
	if _, err := session.Exec("UPDATE articles SET comment_count = 0, like_count = 0"); err != nil {
		return err
	}
	// 2. 按当前评论关系投影恢复评论数
	commentCounts := make([]struct {
		ArticleID uint64 `xorm:"'article_id'"`
		Count     int64  `xorm:"'count'"`
	}, 0)
	if err := session.SQL("SELECT article_id, COUNT(*) AS count FROM article_comment_projection WHERE active = 1 GROUP BY article_id").Find(&commentCounts); err != nil {
		return err
	}
	for _, count := range commentCounts {
		if _, err := session.ID(count.ArticleID).Cols("comment_count").Update(&po.Article{CommentCount: count.Count}); err != nil {
			return err
		}
	}
	// 3. 按当前点赞关系投影恢复点赞数
	likeCounts := make([]struct {
		ArticleID uint64 `xorm:"'article_id'"`
		Count     int64  `xorm:"'count'"`
	}, 0)
	if err := session.SQL("SELECT article_id, COUNT(*) AS count FROM article_like_projection WHERE active = 1 GROUP BY article_id").Find(&likeCounts); err != nil {
		return err
	}
	for _, count := range likeCounts {
		if _, err := session.ID(count.ArticleID).Cols("like_count").Update(&po.Article{LikeCount: count.Count}); err != nil {
			return err
		}
	}
	return nil
}
