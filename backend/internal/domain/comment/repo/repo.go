package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo/factory"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo/po"
	"github.com/google/uuid"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// transactionClient 定义评论写入所需的 MySQL 事务能力。
type transactionClient interface {
	// Transaction 在同一数据库事务中执行评论与回复计数写入。
	Transaction(func(*xorm.Session) (interface{}, error)) (interface{}, error)
}

// ArticleLocker 在评论事务内锁定并校验文章公开状态。
type ArticleLocker interface {
	// LockPublishedArticle 锁定并校验文章处于已发表状态。
	LockPublishedArticle(*xorm.Session, uint64) error
}

// mysqlArticleLocker 使用文章表状态实现评论创建校验。
type mysqlArticleLocker struct{}

// LockPublishedArticle 执行评论上下文对应的处理。
func (mysqlArticleLocker) LockPublishedArticle(session *xorm.Session, articleID uint64) error {
	// 1. 锁定文章行并校验已发表状态
	var row struct {
		Status int8 `xorm:"'status'"` // Status 是文章状态：3-已发表。
	}
	found, err := forUpdate(session.Table("articles").Where("id = ?", articleID)).Cols("status").Get(&row)
	if err != nil {
		return err
	}
	if !found {
		return comment.ErrArticleNotPublished
	}
	if row.Status != comment.ArticleStatusPublished {
		return comment.ErrArticleNotPublished
	}
	return nil
}

// Repository 使用 MySQL 实现评论上下文仓储。
type Repository struct {
	client        clients.MysqlClient // client 提供评论普通查询能力。
	transaction   transactionClient   // transaction 提供评论创建事务能力。
	articleLocker ArticleLocker       // articleLocker 提供事务内文章有效性校验。
	now           func() time.Time    // now 提供可测试的当前时间。
	newEventID    func() string       // newEventID 提供可测试的事件标识。
}

// NewRepository 创建评论 MySQL 仓储。
func NewRepository(client clients.MysqlClient, transaction transactionClient) *Repository {
	// 1. 校验并保存评论 MySQL 与事务依赖
	if client == nil || transaction == nil {
		panic("评论仓储缺少事务数据库客户端")
	}
	return &Repository{client: client, transaction: transaction, articleLocker: mysqlArticleLocker{}, now: time.Now, newEventID: uuid.NewString}
}

// Create 在事务中创建评论并维护根评论回复数。
func (r *Repository) Create(ctx context.Context, commentEntity *entity.Comment) error {
	// 1. 在事务中校验文章和根评论、写入评论并更新回复数
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		if err := r.articleLocker.LockPublishedArticle(session, commentEntity.ArticleID); err != nil {
			return nil, err
		}
		if commentEntity.RootID > 0 {
			root := new(po.Comment)
			found, err := forUpdate(session.Where("id = ?", commentEntity.RootID)).Get(root)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, comment.ErrRootNotFound
			}
			if root.Status != comment.StatusNormal || root.RootID != 0 {
				return nil, comment.ErrRootDeleted
			}
			if root.ArticleID != commentEntity.ArticleID {
				return nil, comment.ErrRootNotFound
			}
		}
		poComment := factory.ToPO(commentEntity)
		if _, err := session.Insert(poComment); err != nil {
			return nil, err
		}
		commentEntity.ID = poComment.ID
		if commentEntity.RootID > 0 {
			if _, err := session.ID(commentEntity.RootID).Incr("comment_count", 1).Update(new(po.Comment)); err != nil {
				return nil, err
			}
		}
		event := comment.IntegrationEvent{EventID: r.eventID(), EventType: comment.CommentCreatedEventType, Version: comment.CommentCreatedVersion, OccurredAt: commentEntity.CreatedTime, AggregateID: commentEntity.ID, CommentID: commentEntity.ID, ArticleID: commentEntity.ArticleID, RootID: commentEntity.RootID}
		if err := r.insertOutbox(session, event); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

// FindByID 查询评论及其删除状态。
func (r *Repository) FindByID(ctx context.Context, id uint64) (*entity.Comment, error) {
	// 1. 保留删除状态以支持重复删除和权限判断
	row := new(po.Comment)
	found, err := r.client.Context(ctx).ID(id).Get(row)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, comment.ErrCommentNotFound
	}
	return factory.FromPO(row), nil
}

// Delete 幂等软删除评论并为实际状态变化写入 Outbox。
func (r *Repository) Delete(ctx context.Context, id uint64) error {
	// 1. 锁定目标评论，已删除记录直接成功返回
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		target := new(po.Comment)
		found, err := forUpdate(session.ID(id)).Get(target)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, comment.ErrCommentNotFound
		}
		if target.Status == comment.StatusDeleted {
			return nil, nil
		}

		// 2. 主评论锁定并删除整楼；回复只删除自身并安全扣减根回复数
		affected := []*po.Comment{target}
		if target.RootID == 0 {
			replies := make([]*po.Comment, 0)
			if err := forUpdate(session.Where("root_id = ? AND status = ?", target.ID, comment.StatusNormal)).Find(&replies); err != nil {
				return nil, err
			}
			affected = append(affected, replies...)
			ids := make([]uint64, 0, len(affected))
			for _, row := range affected {
				ids = append(ids, row.ID)
			}
			if _, err := session.In("id", ids).And("status = ?", comment.StatusNormal).Cols("status").Update(&po.Comment{Status: comment.StatusDeleted}); err != nil {
				return nil, err
			}
			if _, err := session.ID(target.ID).Cols("comment_count").Update(&po.Comment{CommentCount: 0}); err != nil {
				return nil, err
			}
		} else {
			rows, err := session.ID(target.ID).And("status = ?", comment.StatusNormal).Cols("status").Update(&po.Comment{Status: comment.StatusDeleted})
			if err != nil {
				return nil, err
			}
			if rows == 0 {
				return nil, nil
			}
			if _, err := session.Exec("UPDATE comments SET comment_count = CASE WHEN comment_count > 0 THEN comment_count - 1 ELSE 0 END WHERE id = ?", target.RootID); err != nil {
				return nil, err
			}
		}

		// 3. 仅为本次实际删除的评论写入版本2事件
		occurredAt := r.currentTime()
		for _, row := range affected {
			event := comment.IntegrationEvent{EventID: r.eventID(), EventType: comment.CommentDeletedEventType, Version: comment.CommentDeletedVersion, OccurredAt: occurredAt, AggregateID: row.ID, CommentID: row.ID, ArticleID: row.ArticleID, RootID: row.RootID}
			if err := r.insertOutbox(session, event); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

// insertOutbox 将评论事实作为 JSON 写入当前业务事务。
func (r *Repository) insertOutbox(session *xorm.Session, event comment.IntegrationEvent) error {
	// 1. 事件与评论写入共享事务，序列化或插入失败均回滚业务事实
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化评论集成事件: %w", err)
	}
	now := r.currentTime()
	_, err = session.Insert(&po.CommentEventOutbox{EventID: event.EventID, AggregateID: event.CommentID, EventType: event.EventType, Version: event.Version, OccurredAt: event.OccurredAt, Payload: string(payload), Status: 0, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now})
	return err
}

// eventID 创建评论事件幂等标识。
func (r *Repository) eventID() string {
	// 1. 测试直构仓储时回退到安全随机 UUID
	if r.newEventID == nil {
		return uuid.NewString()
	}
	return r.newEventID()
}

// currentTime 返回评论事务使用的当前时间。
func (r *Repository) currentTime() time.Time {
	// 1. 测试直构仓储时使用系统时间
	if r.now == nil {
		return time.Now()
	}
	return r.now()
}

// FindRoot 查询根评论，包括删除状态以便区分列表隐藏和回复拒绝。
func (r *Repository) FindRoot(ctx context.Context, id uint64) (*entity.Comment, error) {
	// 1. 查询指定根评论及其状态
	row := new(po.Comment)
	found, err := r.client.Context(ctx).Where("id = ? AND root_id = 0", id).Get(row)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, comment.ErrRootNotFound
	}
	return factory.FromPO(row), nil
}

// HasReplyTarget 查询用户是否属于根评论作者或其正常直属回复链。
func (r *Repository) HasReplyTarget(ctx context.Context, rootID, userID uint64) (bool, error) {
	// 1. 查询被回复用户是否属于当前根评论链
	count, err := r.client.Context(ctx).Where("root_id = ? AND user_id = ? AND status = ?", rootID, userID, comment.StatusNormal).Count(new(po.Comment))
	return count > 0, err
}

// ListRoots 分页查询正常主评论。
func (r *Repository) ListRoots(ctx context.Context, query comment.RootListQuery) (*comment.ListResult, error) {
	// 1. 构建主评论状态、作者和分页查询
	build := func() *xorm.Session {
		session := r.client.Context(ctx).Where("article_id = ? AND root_id = 0 AND status = ?", query.ArticleID, comment.StatusNormal)
		if query.AuthorID > 0 {
			session = session.And("user_id = ?", query.AuthorID)
		}
		return session
	}
	return r.list(build, query.PageQuery)
}

// ListReplies 分页查询正常直属回复。
func (r *Repository) ListReplies(ctx context.Context, rootID uint64, query comment.PageQuery) (*comment.ListResult, error) {
	// 1. 构建直属回复状态和分页查询
	build := func() *xorm.Session {
		return r.client.Context(ctx).Where("root_id = ? AND status = ?", rootID, comment.StatusNormal)
	}
	return r.list(build, query)
}

// list 执行评论上下文对应的处理。
func (r *Repository) list(build func() *xorm.Session, query comment.PageQuery) (*comment.ListResult, error) {
	// 1. 统计总数并按游标或 Offset 查询评论
	total, err := build().Count(new(po.Comment))
	if err != nil {
		return nil, err
	}
	order := "id ASC"
	operator := ">"
	if query.IsDesc {
		order = "id DESC"
		operator = "<"
	}
	session := build()
	if query.LastID > 0 {
		session = session.And("id "+operator+" ?", query.LastID).Limit(int(query.PageSize))
	} else {
		session = session.Limit(int(query.PageSize), int((query.Page-1)*query.PageSize))
	}
	rows := make([]*po.Comment, 0, query.PageSize)
	if err := session.OrderBy(order).Find(&rows); err != nil {
		return nil, err
	}
	items := make([]*entity.Item, 0, len(rows))
	var lastID uint64
	for _, row := range rows {
		lastID = row.ID
		items = append(items, &entity.Item{Comment: factory.FromPO(row)})
	}
	return &comment.ListResult{Items: items, LastID: lastID, Total: uint64(total), Page: query.Page, PageSize: query.PageSize}, nil
}

// forUpdate 执行评论上下文对应的处理。
func forUpdate(session *xorm.Session) *xorm.Session {
	// 1. 仅在 MySQL 方言启用行锁
	if session.Engine().Dialect().URI().DBType == schemas.MYSQL {
		return session.ForUpdate()
	}
	return session
}

// ProvideTransactionClient 提供评论仓储需要的事务能力。
func ProvideTransactionClient(client clients.MysqlClient) transactionClient {
	// 1. 校验并暴露评论 MySQL 事务能力
	transaction, ok := client.(transactionClient)
	if !ok {
		panic("评论仓储要求 MySQL 客户端支持事务")
	}
	return transaction
}

// ListPending 查询到期且尚未发布的评论事件。
func (r *Repository) ListPending(ctx context.Context, limit int, now time.Time) ([]comment.OutboxMessage, error) {
	// 1. 按创建顺序读取有限数量待发布消息
	if limit <= 0 {
		return nil, nil
	}
	rows := make([]*po.CommentEventOutbox, 0, limit)
	if err := r.client.Context(ctx).Where("status = 0 AND next_attempt_time <= ?", now).OrderBy("created_time ASC").Limit(limit).Find(&rows); err != nil {
		return nil, err
	}
	messages := make([]comment.OutboxMessage, 0, len(rows))
	for _, row := range rows {
		var event comment.IntegrationEvent
		if err := json.Unmarshal([]byte(row.Payload), &event); err != nil {
			return nil, fmt.Errorf("解析评论 Outbox 事件 %s: %w", row.EventID, err)
		}
		messages = append(messages, comment.OutboxMessage{Event: event, Attempts: row.Attempts, NextAttempt: row.NextAttemptAt})
	}
	return messages, nil
}

// MarkPublished 将评论事件标记为发布完成。
func (r *Repository) MarkPublished(ctx context.Context, eventID string, publishedAt time.Time) error {
	// 1. 只允许待发布状态转换为完成，重复确认保持幂等
	_, err := r.client.Context(ctx).Where("event_id = ? AND status = 0", eventID).Cols("status", "published_time", "last_error", "updated_time").Update(&po.CommentEventOutbox{Status: 1, PublishedAt: &publishedAt, LastError: "", UpdatedAt: publishedAt})
	return err
}

// MarkFailed 记录评论事件发布失败并安排下次重试。
func (r *Repository) MarkFailed(ctx context.Context, eventID string, cause string, nextAttempt time.Time) error {
	// 1. 已发布消息不能被迟到失败结果改回待发布状态
	_, err := r.client.Context(ctx).Where("event_id = ? AND status = 0", eventID).Incr("attempts", 1).Cols("last_error", "next_attempt_time", "updated_time").Update(&po.CommentEventOutbox{LastError: cause, NextAttemptAt: nextAttempt, UpdatedAt: time.Now()})
	return err
}
