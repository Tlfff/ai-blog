package repo

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// articleCommentProjection 保存单条评论对文章计数的当前贡献状态。
type articleCommentProjection struct {
	CommentID   uint64    `xorm:"'comment_id' pk"`             // CommentID 是评论标识。
	ArticleID   uint64    `xorm:"'article_id'"`                // ArticleID 是所属文章标识。
	Version     int64     `xorm:"'version'"`                   // Version 是最后应用的评论聚合版本。
	Active      int8      `xorm:"'active'"`                    // Active 表示评论是否计入文章评论数：0-否；1-是。
	LastEventID string    `xorm:"'last_event_id' varchar(64)"` // LastEventID 是最后应用的事件标识。
	UpdatedTime time.Time `xorm:"'updated_time' datetime"`     // UpdatedTime 是投影更新时间。
}

// TableName 返回文章评论状态投影表名。
func (articleCommentProjection) TableName() string {
	// 1. 返回文章上下文拥有的评论状态投影表
	return "article_comment_projection"
}

// ApplyCommentCountEvent 原子处理 Inbox、评论状态和文章计数。
func (r *Repository) ApplyCommentCountEvent(ctx context.Context, event article.CommentCountEvent) error {
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
		inserted, err := insertCommentEventInbox(session, event)
		if err != nil || !inserted {
			return nil, err
		}

		// 2. 聚合版本不新于当前状态时仅保留 Inbox，不重复改变计数
		current := new(articleCommentProjection)
		found, err = forUpdate(session.ID(event.CommentID)).Get(current)
		if err != nil {
			return nil, err
		}
		if found && current.ArticleID != event.ArticleID {
			return nil, article.ErrInvalidCommentCountEvent
		}
		if found && current.Version >= event.Version {
			return nil, nil
		}

		// 3. 只在评论是否计数的状态发生转换时调整文章评论数
		desiredActive := int8(0)
		if event.EventType == article.CommentCreatedEventType {
			desiredActive = 1
		}
		currentActive := int8(0)
		if found {
			currentActive = current.Active
		}
		delta := desiredActive - currentActive
		if delta > 0 {
			if _, err := session.Exec("UPDATE articles SET comment_count = comment_count + 1 WHERE id = ?", event.ArticleID); err != nil {
				return nil, err
			}
		} else if delta < 0 {
			if _, err := session.Exec("UPDATE articles SET comment_count = CASE WHEN comment_count > 0 THEN comment_count - 1 ELSE 0 END WHERE id = ?", event.ArticleID); err != nil {
				return nil, err
			}
		}

		// 4. 覆盖该评论的最新版本，使删除先到时能够屏蔽迟到创建事件
		now := time.Now()
		projection := &articleCommentProjection{CommentID: event.CommentID, ArticleID: event.ArticleID, Version: event.Version, Active: desiredActive, LastEventID: event.EventID, UpdatedTime: now}
		if found {
			_, err = session.ID(event.CommentID).AllCols().Update(projection)
		} else {
			_, err = session.Insert(projection)
		}
		return nil, err
	})
	return err
}

// insertCommentEventInbox 登记事件并返回是否由当前事务首次插入。
func insertCommentEventInbox(session *xorm.Session, event article.CommentCountEvent) (bool, error) {
	// 1. 使用数据库方言的忽略冲突语法，以事件标识实现消息幂等
	query := "INSERT IGNORE INTO article_comment_event_inbox (event_id, comment_id, article_id, processed_time) VALUES (?, ?, ?, ?)"
	if session.Engine().Dialect().URI().DBType != schemas.MYSQL {
		query = "INSERT OR IGNORE INTO article_comment_event_inbox (event_id, comment_id, article_id, processed_time) VALUES (?, ?, ?, ?)"
	}
	result, err := session.Exec(query, event.EventID, event.CommentID, event.ArticleID, time.Now())
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}
