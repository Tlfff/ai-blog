package repo

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// commentLikeProjection 保存单条点赞关系对评论计数的当前贡献状态。
type commentLikeProjection struct {
	LikeID      uint64    `xorm:"'like_id' pk"`                // LikeID 是点赞关系标识。
	CommentID   uint64    `xorm:"'comment_id'"`                // CommentID 是所属评论标识。
	UserID      uint64    `xorm:"'user_id'"`                   // UserID 是点赞用户标识。
	Version     int64     `xorm:"'version'"`                   // Version 是最后应用的点赞关系版本。
	Active      int8      `xorm:"'active'"`                    // Active 表示该关系是否计入评论点赞数。
	LastEventID string    `xorm:"'last_event_id' varchar(64)"` // LastEventID 是最后应用的事件标识。
	UpdatedTime time.Time `xorm:"'updated_time' datetime"`     // UpdatedTime 是投影更新时间。
}

// TableName 返回评论点赞状态投影表名。
func (commentLikeProjection) TableName() string { return "comment_like_projection" }

// ApplyLikeCountEvent 原子处理评论 Inbox、点赞状态和评论计数。
func (r *Repository) ApplyLikeCountEvent(ctx context.Context, event comment.LikeCountEvent) error {
	// 1. 在同一事务中锁定评论并登记事件 Inbox
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		if err := lockCommentLikeProjection(session); err != nil {
			return nil, err
		}
		var commentRow struct {
			ID uint64 `xorm:"'id'"` // ID 是评论标识。
		}
		found, err := forUpdate(session.Table("comments").Where("id = ?", event.CommentID)).Cols("id").Get(&commentRow)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, comment.ErrCommentNotFound
		}
		inserted, err := insertCommentLikeEventInbox(session, event)
		if err != nil || !inserted {
			return nil, err
		}

		// 2. 不新于当前关系版本的消息仅保留 Inbox，不重复改变计数
		current := new(commentLikeProjection)
		found, err = forUpdate(session.ID(event.LikeID)).Get(current)
		if err != nil {
			return nil, err
		}
		if found && (current.CommentID != event.CommentID || current.UserID != event.UserID) {
			return nil, comment.ErrInvalidLikeCountEvent
		}
		if found && current.Version >= event.Version {
			return nil, nil
		}

		// 3. 只在点赞关系最终状态转换时调整评论点赞数，取消时保证不为负
		desiredActive := int8(0)
		if event.EventType == comment.CommentLikedEventType {
			desiredActive = 1
		}
		currentActive := int8(0)
		if found {
			currentActive = current.Active
		}
		delta := desiredActive - currentActive
		if delta > 0 {
			if _, err := session.Exec("UPDATE comments SET like_count = like_count + 1 WHERE id = ?", event.CommentID); err != nil {
				return nil, err
			}
		} else if delta < 0 {
			if _, err := session.Exec("UPDATE comments SET like_count = CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END WHERE id = ?", event.CommentID); err != nil {
				return nil, err
			}
		}

		// 4. 覆盖点赞关系最新版本，使取消先到时屏蔽迟到点赞事件
		now := time.Now()
		projection := &commentLikeProjection{LikeID: event.LikeID, CommentID: event.CommentID, UserID: event.UserID, Version: event.Version, Active: desiredActive, LastEventID: event.EventID, UpdatedTime: now}
		if found {
			_, err = session.ID(event.LikeID).AllCols().Update(projection)
		} else {
			_, err = session.Insert(projection)
		}
		return nil, err
	})
	return err
}

// insertCommentLikeEventInbox 登记评论点赞事件并返回是否首次插入。
func insertCommentLikeEventInbox(session *xorm.Session, event comment.LikeCountEvent) (bool, error) {
	// 1. 使用数据库方言的忽略冲突语法，以事件标识实现消息幂等
	query := "INSERT IGNORE INTO comment_like_event_inbox (event_id, like_id, comment_id, processed_time) VALUES (?, ?, ?, ?)"
	if session.Engine().Dialect().URI().DBType != schemas.MYSQL {
		query = "INSERT OR IGNORE INTO comment_like_event_inbox (event_id, like_id, comment_id, processed_time) VALUES (?, ?, ?, ?)"
	}
	result, err := session.Exec(query, event.EventID, event.LikeID, event.CommentID, time.Now())
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// RebuildLikeCountProjection 从评论点赞事实重建关系投影和评论点赞数。
func (r *Repository) RebuildLikeCountProjection(ctx context.Context) error {
	// 1. 在单个事务中读取每条关系的最终状态和最后事件版本
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		if err := lockCommentLikeProjection(session); err != nil {
			return nil, err
		}
		facts := make([]struct {
			LikeID    uint64 `xorm:"'like_id'"`    // LikeID 是点赞关系标识。
			CommentID uint64 `xorm:"'comment_id'"` // CommentID 是评论标识。
			UserID    uint64 `xorm:"'user_id'"`    // UserID 是点赞用户标识。
			Status    int8   `xorm:"'status'"`     // Status 是当前点赞事实状态。
			Version   int64  `xorm:"'version'"`    // Version 是最后事件版本。
		}, 0)
		query := `SELECT cl.id AS like_id, cl.comment_id, cl.user_id, cl.status, COALESCE(MAX(o.version), 0) AS version
FROM comment_likes cl
LEFT JOIN comment_like_event_outbox o ON o.aggregate_id = cl.id
GROUP BY cl.id, cl.comment_id, cl.user_id, cl.status
ORDER BY cl.id`
		if err := session.SQL(query).Find(&facts); err != nil {
			return nil, err
		}

		// 2. 清空可重建关系投影和评论计数，再按权威事实完整覆盖
		if _, err := session.Exec("DELETE FROM comment_like_projection"); err != nil {
			return nil, err
		}
		if _, err := session.Exec("UPDATE comments SET like_count = 0"); err != nil {
			return nil, err
		}
		now := time.Now()
		for _, fact := range facts {
			active := int8(0)
			if fact.Status == like.StatusLiked {
				active = 1
			}
			projection := &commentLikeProjection{LikeID: fact.LikeID, CommentID: fact.CommentID, UserID: fact.UserID, Version: fact.Version, Active: active, LastEventID: "rebuild", UpdatedTime: now}
			if _, err := session.Insert(projection); err != nil {
				return nil, err
			}
			if active == 1 {
				if _, err := session.Exec("UPDATE comments SET like_count = like_count + 1 WHERE id = ?", fact.CommentID); err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	})
	return err
}

// lockCommentLikeProjection 串行化事件应用与完整重建，避免重建覆盖并发事件结果。
func lockCommentLikeProjection(session *xorm.Session) error {
	// 1. 锁定单例协调行，保证事件事务和重建事务互斥
	var row struct {
		ID int `xorm:"'id'"`
	}
	found, err := forUpdate(session.Table("comment_like_projection_lock").Where("id = 1")).Get(&row)
	if err != nil {
		return err
	}
	if !found {
		return comment.ErrInvalidLikeCountEvent
	}
	return nil
}
