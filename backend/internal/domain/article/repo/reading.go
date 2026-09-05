package repo

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo/factory"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo/po"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// ListPublished 分页查询已发表文章。
func (r *Repository) ListPublished(ctx context.Context, query article.PublicListQuery) ([]*entity.Article, uint64, error) {
	// 1. 统计已发表文章总数
	total, err := r.client.Context(ctx).Where("status = ?", article.StatusPublished).Count(new(po.Article))
	if err != nil {
		return nil, 0, err
	}

	// 2. last_id 大于零时优先使用游标，否则按页码计算 Offset
	session := r.client.Context(ctx).Where("status = ?", article.StatusPublished)
	order := "id ASC"
	if query.IsDesc {
		order = "id DESC"
	}
	if query.LastID > 0 {
		operator := ">"
		if query.IsDesc {
			operator = "<"
		}
		session = session.And("id "+operator+" ?", query.LastID).Limit(int(query.PageSize))
	} else {
		offset := int((query.Page - 1) * query.PageSize)
		session = session.Limit(int(query.PageSize), offset)
	}

	// 3. 使用稳定 ID 排序并转换领域文章
	articlePOs := make([]*po.Article, 0, query.PageSize)
	if err := session.OrderBy(order).Find(&articlePOs); err != nil {
		return nil, 0, err
	}
	articles := make([]*entity.Article, 0, len(articlePOs))
	for _, articlePO := range articlePOs {
		articles = append(articles, factory.ArticleFromPO(articlePO))
	}
	return articles, uint64(total), nil
}

// RecordView 原子维护登录用户历史并增加文章浏览量。
func (r *Repository) RecordView(ctx context.Context, event article.ViewEvent) (*article.HotMetric, error) {
	// 1. 锁定已发表文章，串行维护同一文章的历史和浏览量
	var metric *article.HotMetric
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		inserted, err := insertViewEventInbox(session, event)
		if err != nil {
			return nil, err
		}
		articlePO, err := findArticleForUpdate(session, event.ArticleID)
		if err != nil {
			return nil, err
		}
		if articlePO.Status != article.StatusPublished {
			return nil, article.ErrArticleNotPublished
		}
		if !inserted {
			metric = hotMetricFromPO(articlePO)
			return nil, nil
		}

		// 2. 登录用户更新既有历史，首次浏览时创建记录；游客不保存历史
		if event.UserID > 0 {
			history := new(po.ViewHistory)
			found, err := session.Where("user_id = ? AND article_id = ?", event.UserID, event.ArticleID).Get(history)
			if err != nil {
				return nil, err
			}
			if found {
				if _, err := session.ID(history.ID).Cols("updated_time").Update(&po.ViewHistory{UpdatedTime: event.ViewedAt}); err != nil {
					return nil, err
				}
			} else if _, err := session.Insert(&po.ViewHistory{UserID: event.UserID, ArticleID: event.ArticleID, CreatedTime: event.ViewedAt, UpdatedTime: event.ViewedAt}); err != nil {
				return nil, err
			}
		}

		// 3. 无论是否登录均增加一次权威浏览量
		if _, err := session.ID(event.ArticleID).Incr("view_count", 1).Update(new(po.Article)); err != nil {
			return nil, err
		}
		metric = hotMetricFromPO(articlePO)
		metric.ViewCount++
		return nil, nil
	})
	return metric, err
}

// ViewEventProcessed 查询浏览事件是否已在 MySQL 事务中完成。
func (r *Repository) ViewEventProcessed(ctx context.Context, eventID string) (bool, error) {
	// 1. Inbox 主键存在即表示历史和浏览量事务已经提交
	return r.client.Context(ctx).ID(eventID).Exist(new(po.ViewEventInbox))
}

// FindHotMetric 查询指定已发表文章的权威热度字段。
func (r *Repository) FindHotMetric(ctx context.Context, articleID uint64) (*article.HotMetric, error) {
	// 1. 只读取已发表文章的标题和互动统计
	articlePO := new(po.Article)
	found, err := r.client.Context(ctx).Where("id = ? AND status = ?", articleID, article.StatusPublished).Get(articlePO)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, article.ErrArticleNotFound
	}
	return hotMetricFromPO(articlePO), nil
}

// FindHotMetrics 批量查询指定已发表文章的权威热度字段。
func (r *Repository) FindHotMetrics(ctx context.Context, articleIDs []uint64) ([]*article.HotMetric, error) {
	// 1. 空排名不触发数据库查询
	if len(articleIDs) == 0 {
		return []*article.HotMetric{}, nil
	}

	// 2. 只返回仍存在且已发表的文章
	articlePOs := make([]*po.Article, 0, len(articleIDs))
	if err := r.client.Context(ctx).In("id", articleIDs).And("status = ?", article.StatusPublished).Find(&articlePOs); err != nil {
		return nil, err
	}
	metrics := make([]*article.HotMetric, 0, len(articlePOs))
	for _, articlePO := range articlePOs {
		metrics = append(metrics, hotMetricFromPO(articlePO))
	}
	return metrics, nil
}

// TopHotMetrics 查询热度最高的已发表文章。
func (r *Repository) TopHotMetrics(ctx context.Context, limit int) ([]*article.HotMetric, error) {
	// 1. 按权威热度公式倒序查询，并以 ID 倒序稳定打破同分
	articlePOs := make([]*po.Article, 0, limit)
	if err := r.client.Context(ctx).
		Where("status = ?", article.StatusPublished).
		OrderBy("(view_count + like_count + comment_count) DESC, id DESC").
		Limit(limit).
		Find(&articlePOs); err != nil {
		return nil, err
	}
	metrics := make([]*article.HotMetric, 0, len(articlePOs))
	for _, articlePO := range articlePOs {
		metrics = append(metrics, hotMetricFromPO(articlePO))
	}
	return metrics, nil
}

// hotMetricFromPO 将文章持久化统计转换为热榜领域数据。
func hotMetricFromPO(articlePO *po.Article) *article.HotMetric {
	// 1. 只复制热榜和浏览投影需要的字段
	return &article.HotMetric{
		ArticleID: articlePO.ID, Title: articlePO.Title, ViewCount: articlePO.ViewCount,
		LikeCount: articlePO.LikeCount, CommentCount: articlePO.CommentCount,
	}
}

// insertViewEventInbox 原子插入浏览事件 Inbox 并返回是否首次处理。
func insertViewEventInbox(session *xorm.Session, event article.ViewEvent) (bool, error) {
	// 1. 使用各数据库方言的忽略冲突语法，主键保证并发幂等
	query := "INSERT IGNORE INTO article_view_event_inbox (event_id, article_id, processed_time) VALUES (?, ?, ?)"
	if session.Engine().Dialect().URI().DBType == schemas.SQLITE {
		query = "INSERT OR IGNORE INTO article_view_event_inbox (event_id, article_id, processed_time) VALUES (?, ?, ?)"
	}
	result, err := session.Exec(query, event.EventID, event.ArticleID, time.Now())
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}
