package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/repo/po"
	"github.com/google/uuid"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// transactionClient 定义点赞事实与 Outbox 原子写入所需的事务能力。
type transactionClient interface {
	// Transaction 在同一数据库事务中执行点赞事实与事件写入。
	Transaction(func(*xorm.Session) (interface{}, error)) (interface{}, error)
}

// Repository 使用 MySQL 实现文章点赞事实和 Outbox。
type Repository struct {
	client      clients.MysqlClient // client 提供点赞事实和 Outbox 查询能力。
	transaction transactionClient   // transaction 提供点赞事实与 Outbox 原子提交能力。
	now         func() time.Time    // now 提供可测试的持久化时间。
	newEventID  func() string       // newEventID 提供可测试的事件标识。
}

// NewRepository 创建点赞 MySQL 仓储。
func NewRepository(client clients.MysqlClient, transaction transactionClient) *Repository {
	// 1. 启动阶段拒绝缺少 MySQL 或事务能力
	if client == nil || transaction == nil {
		panic("点赞仓储缺少事务数据库客户端")
	}
	return &Repository{client: client, transaction: transaction, now: time.Now, newEventID: uuid.NewString}
}

// ChangeArticleLike 幂等变更点赞事实，并为实际状态变化原子写入 Outbox。
//
// 参数说明：
//   - ctx：当前请求上下文。
//   - userID：点赞用户标识。
//   - articleID：目标文章标识。
//   - desiredStatus：请求完成后的点赞最终状态。
//   - occurredAt：事实变更时间。
func (r *Repository) ChangeArticleLike(ctx context.Context, userID, articleID uint64, desiredStatus int8, occurredAt time.Time) (bool, error) {
	// 1. 只接受点赞上下文定义的两种最终状态
	if userID == 0 || articleID == 0 || (desiredStatus != like.StatusLiked && desiredStatus != like.StatusUnliked) {
		return false, like.ErrInvalidInput
	}
	if occurredAt.IsZero() {
		occurredAt = r.currentTime()
	}

	// 2. 使用唯一关系键和行锁串行化同一用户对同一文章的状态转换
	changed := false
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		row, inserted, err := r.findOrCreateLike(session, userID, articleID, desiredStatus, occurredAt)
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, nil
		}
		if !inserted && row.Status == desiredStatus {
			return nil, nil
		}
		if !inserted {
			row.Status = desiredStatus
			row.UpdatedTime = occurredAt
			if _, err := session.ID(row.ID).Cols("status", "updated_time").Update(row); err != nil {
				return nil, err
			}
		}

		// 3. 每次真实状态变化生成单调版本，并与点赞事实在同一事务写入
		version, err := nextLikeVersion(session, row.ID)
		if err != nil {
			return nil, err
		}
		eventType := like.ArticleLikedEventType
		if desiredStatus == like.StatusUnliked {
			eventType = like.ArticleUnlikedEventType
		}
		event := like.IntegrationEvent{EventID: r.eventID(), EventType: eventType, Version: version, OccurredAt: occurredAt, AggregateID: row.ID, LikeID: row.ID, ArticleID: articleID, UserID: userID}
		if err := r.insertOutbox(session, event); err != nil {
			return nil, err
		}
		changed = true
		return nil, nil
	})
	return changed, err
}

// findOrCreateLike 锁定现有关系；首次点赞时可直接创建有效关系。
//
// 参数说明：
//   - session：当前点赞事实事务。
//   - userID：点赞用户标识。
//   - articleID：目标文章标识。
//   - desiredStatus：请求完成后的点赞最终状态。
//   - occurredAt：事实变更时间。
func (r *Repository) findOrCreateLike(session *xorm.Session, userID, articleID uint64, desiredStatus int8, occurredAt time.Time) (*po.ArticleLike, bool, error) {
	// 1. 从未点赞过的重复取消不创建无意义事实记录
	if desiredStatus == like.StatusUnliked {
		row := new(po.ArticleLike)
		found, err := forUpdate(session.Where("user_id = ? AND article_id = ?", userID, articleID)).Get(row)
		if err != nil || !found {
			return nil, false, err
		}
		return row, false, nil
	}

	// 2. INSERT IGNORE 结合唯一索引安全处理并发首次点赞
	query := "INSERT IGNORE INTO article_likes (user_id, article_id, status, created_time, updated_time) VALUES (?, ?, ?, ?, ?)"
	if session.Engine().Dialect().URI().DBType != schemas.MYSQL {
		query = "INSERT OR IGNORE INTO article_likes (user_id, article_id, status, created_time, updated_time) VALUES (?, ?, ?, ?, ?)"
	}
	result, err := session.Exec(query, userID, articleID, like.StatusLiked, occurredAt, occurredAt)
	if err != nil {
		return nil, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, false, err
	}
	row := new(po.ArticleLike)
	found, err := forUpdate(session.Where("user_id = ? AND article_id = ?", userID, articleID)).Get(row)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, fmt.Errorf("创建文章点赞关系后未找到记录")
	}
	return row, rows > 0, nil
}

// nextLikeVersion 查询同一点赞关系的下一事件版本。
func nextLikeVersion(session *xorm.Session, likeID uint64) (int64, error) {
	// 1. 点赞关系行锁保证同一聚合的版本查询和写入串行执行
	var latest struct {
		Version int64 `xorm:"'version'"` // Version 是当前最大事件版本。
	}
	found, err := session.Table("article_like_event_outbox").Where("aggregate_id = ?", likeID).Desc("version").Cols("version").Get(&latest)
	if err != nil {
		return 0, err
	}
	if !found {
		return 1, nil
	}
	return latest.Version + 1, nil
}

// insertOutbox 将点赞事件写入当前事实事务。
func (r *Repository) insertOutbox(session *xorm.Session, event like.IntegrationEvent) error {
	// 1. 保存完整稳定负载，Publisher 重试时不重新组装业务事实
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	now := r.currentTime()
	_, err = session.Insert(&po.ArticleLikeEventOutbox{EventID: event.EventID, AggregateID: event.AggregateID, EventType: event.EventType, Version: event.Version, OccurredAt: event.OccurredAt, Payload: string(payload), NextAttemptAt: now, CreatedAt: now, UpdatedAt: now})
	return err
}

// ListActiveArticleLikes 查询全部当前生效的文章点赞事实。
func (r *Repository) ListActiveArticleLikes(ctx context.Context) ([]*entity.ArticleLike, error) {
	// 1. 按关系标识稳定读取 MySQL 权威事实
	rows := make([]*po.ArticleLike, 0)
	if err := r.client.Context(ctx).Where("status = ?", like.StatusLiked).Asc("id").Find(&rows); err != nil {
		return nil, err
	}
	facts := make([]*entity.ArticleLike, 0, len(rows))
	for _, row := range rows {
		facts = append(facts, &entity.ArticleLike{ID: row.ID, UserID: row.UserID, ArticleID: row.ArticleID, Status: row.Status, CreatedTime: row.CreatedTime, UpdatedTime: row.UpdatedTime})
	}
	return facts, nil
}

// ListPending 查询到期且尚未发布的点赞事件。
func (r *Repository) ListPending(ctx context.Context, limit int, now time.Time) ([]like.OutboxMessage, error) {
	// 1. 按创建顺序读取有限数量待发布消息
	if limit <= 0 {
		return nil, nil
	}
	rows := make([]*po.ArticleLikeEventOutbox, 0, limit)
	if err := r.client.Context(ctx).Where("status = 0 AND next_attempt_time <= ?", now).OrderBy("created_time ASC").Limit(limit).Find(&rows); err != nil {
		return nil, err
	}
	messages := make([]like.OutboxMessage, 0, len(rows))
	for _, row := range rows {
		var event like.IntegrationEvent
		if err := json.Unmarshal([]byte(row.Payload), &event); err != nil {
			return nil, fmt.Errorf("解析点赞 Outbox 事件 %s: %w", row.EventID, err)
		}
		messages = append(messages, like.OutboxMessage{Event: event, Attempts: row.Attempts, NextAttempt: row.NextAttemptAt})
	}
	return messages, nil
}

// MarkPublished 将点赞事件标记为发布完成。
func (r *Repository) MarkPublished(ctx context.Context, eventID string, publishedAt time.Time) error {
	// 1. 只允许待发布状态转换为完成，重复确认保持幂等
	_, err := r.client.Context(ctx).Where("event_id = ? AND status = 0", eventID).Cols("status", "published_time", "last_error", "updated_time").Update(&po.ArticleLikeEventOutbox{Status: 1, PublishedAt: &publishedAt, LastError: "", UpdatedAt: publishedAt})
	return err
}

// MarkFailed 记录点赞事件发布失败并安排下次重试。
func (r *Repository) MarkFailed(ctx context.Context, eventID string, cause string, nextAttempt time.Time) error {
	// 1. 已发布消息不能被迟到失败结果改回待发布状态
	_, err := r.client.Context(ctx).Where("event_id = ? AND status = 0", eventID).Incr("attempts", 1).Cols("last_error", "next_attempt_time", "updated_time").Update(&po.ArticleLikeEventOutbox{LastError: cause, NextAttemptAt: nextAttempt, UpdatedAt: r.currentTime()})
	return err
}

// currentTime 返回当前持久化时间。
func (r *Repository) currentTime() time.Time {
	// 1. 测试可替换时钟，生产默认使用系统时间
	if r.now == nil {
		return time.Now()
	}
	return r.now()
}

// eventID 返回新的稳定事件标识。
func (r *Repository) eventID() string {
	// 1. 测试可替换生成器，生产默认使用 UUID
	if r.newEventID == nil {
		return uuid.NewString()
	}
	return r.newEventID()
}

// forUpdate 对 MySQL 查询启用行锁。
func forUpdate(session *xorm.Session) *xorm.Session {
	// 1. SQLite 测试方言不支持 FOR UPDATE
	if session.Engine().Dialect().URI().DBType == schemas.MYSQL {
		return session.ForUpdate()
	}
	return session
}

// ProvideTransactionClient 提供点赞仓储需要的事务能力。
func ProvideTransactionClient(client clients.MysqlClient) transactionClient {
	// 1. 校验并暴露点赞 MySQL 事务能力
	transaction, ok := client.(transactionClient)
	if !ok {
		panic("点赞仓储要求 MySQL 客户端支持事务")
	}
	return transaction
}
