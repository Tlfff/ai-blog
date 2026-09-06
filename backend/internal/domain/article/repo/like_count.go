package repo

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// articleLikeProjection 保存单条点赞关系对文章计数的当前贡献状态。
type articleLikeProjection struct {
	LikeID      uint64    `xorm:"'like_id' pk"`                // LikeID 是点赞关系标识。
	ArticleID   uint64    `xorm:"'article_id'"`                // ArticleID 是所属文章标识。
	UserID      uint64    `xorm:"'user_id'"`                   // UserID 是点赞用户标识。
	Version     int64     `xorm:"'version'"`                   // Version 是最后应用的点赞关系版本。
	Active      int8      `xorm:"'active'"`                    // Active 表示该关系是否计入文章点赞数。
	LastEventID string    `xorm:"'last_event_id' varchar(64)"` // LastEventID 是最后应用的事件标识。
	UpdatedTime time.Time `xorm:"'updated_time' datetime"`     // UpdatedTime 是投影更新时间。
}

// TableName 返回文章点赞状态投影表名。
func (articleLikeProjection) TableName() string { return "article_like_projection" }

// ApplyLikeCountEvent 原子处理 Inbox、点赞状态和文章计数。
func (r *Repository) ApplyLikeCountEvent(ctx context.Context, event article.LikeCountEvent) error {
	// 1. 在同一事务中锁定文章并登记事件 Inbox
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		var articleRow struct {
			ID uint64 `xorm:"'id'"` // ID 是文章标识。
		}
		found, err := forUpdate(session.Table("articles").Where("id = ?", event.ArticleID)).Cols("id").Get(&articleRow)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, article.ErrArticleNotFound
		}
		inserted, err := insertLikeEventInbox(session, event)
		if err != nil || !inserted {
			return nil, err
		}

		// 2. 不新于当前关系版本的消息仅保留 Inbox，不重复改变计数
		current := new(articleLikeProjection)
		found, err = forUpdate(session.ID(event.LikeID)).Get(current)
		if err != nil {
			return nil, err
		}
		if found && (current.ArticleID != event.ArticleID || current.UserID != event.UserID) {
			return nil, article.ErrInvalidLikeCountEvent
		}
		if found && current.Version >= event.Version {
			return nil, nil
		}

		// 3. 只在点赞关系最终状态转换时调整文章点赞数
		desiredActive := int8(0)
		if event.EventType == article.ArticleLikedEventType {
			desiredActive = 1
		}
		currentActive := int8(0)
		if found {
			currentActive = current.Active
		}
		delta := desiredActive - currentActive
		if delta > 0 {
			if _, err := session.Exec("UPDATE articles SET like_count = like_count + 1 WHERE id = ?", event.ArticleID); err != nil {
				return nil, err
			}
		} else if delta < 0 {
			if _, err := session.Exec("UPDATE articles SET like_count = CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END WHERE id = ?", event.ArticleID); err != nil {
				return nil, err
			}
		}

		// 4. 覆盖点赞关系最新版本，使取消先到时屏蔽迟到点赞事件
		now := time.Now()
		projection := &articleLikeProjection{LikeID: event.LikeID, ArticleID: event.ArticleID, UserID: event.UserID, Version: event.Version, Active: desiredActive, LastEventID: event.EventID, UpdatedTime: now}
		if found {
			_, err = session.ID(event.LikeID).AllCols().Update(projection)
		} else {
			_, err = session.Insert(projection)
		}
		return nil, err
	})
	return err
}

// insertLikeEventInbox 登记事件并返回是否由当前事务首次插入。
func insertLikeEventInbox(session *xorm.Session, event article.LikeCountEvent) (bool, error) {
	// 1. 使用数据库方言的忽略冲突语法，以事件标识实现消息幂等
	query := "INSERT IGNORE INTO article_like_event_inbox (event_id, like_id, article_id, processed_time) VALUES (?, ?, ?, ?)"
	if session.Engine().Dialect().URI().DBType != schemas.MYSQL {
		query = "INSERT OR IGNORE INTO article_like_event_inbox (event_id, like_id, article_id, processed_time) VALUES (?, ?, ?, ?)"
	}
	result, err := session.Exec(query, event.EventID, event.LikeID, event.ArticleID, time.Now())
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}
