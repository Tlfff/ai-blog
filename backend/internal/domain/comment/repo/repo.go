package repo

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo/factory"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo/po"
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
	// 1. 执行当前评论处理阶段
	var row struct {
		Status int8 `xorm:"'status'"`
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
}

// NewRepository 创建评论 MySQL 仓储。
func NewRepository(client clients.MysqlClient, transaction transactionClient) *Repository {
	// 1. 执行当前评论处理阶段
	if client == nil || transaction == nil {
		panic("评论仓储缺少事务数据库客户端")
	}
	return &Repository{client: client, transaction: transaction, articleLocker: mysqlArticleLocker{}}
}

// Create 在事务中创建评论并维护根评论回复数。
func (r *Repository) Create(ctx context.Context, commentEntity *entity.Comment) error {
	// 1. 执行当前评论处理阶段
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
		return nil, nil
	})
	return err
}

// FindRoot 查询根评论，包括删除状态以便区分列表隐藏和回复拒绝。
func (r *Repository) FindRoot(ctx context.Context, id uint64) (*entity.Comment, error) {
	// 1. 执行当前评论处理阶段
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
	// 1. 执行当前评论处理阶段
	count, err := r.client.Context(ctx).Where("root_id = ? AND user_id = ? AND status = ?", rootID, userID, comment.StatusNormal).Count(new(po.Comment))
	return count > 0, err
}

// ListRoots 分页查询正常主评论。
func (r *Repository) ListRoots(ctx context.Context, query comment.RootListQuery) (*comment.ListResult, error) {
	// 1. 执行当前评论处理阶段
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
	// 1. 执行当前评论处理阶段
	build := func() *xorm.Session {
		return r.client.Context(ctx).Where("root_id = ? AND status = ?", rootID, comment.StatusNormal)
	}
	return r.list(build, query)
}

// list 执行评论上下文对应的处理。
func (r *Repository) list(build func() *xorm.Session, query comment.PageQuery) (*comment.ListResult, error) {
	// 1. 执行当前评论处理阶段
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
	// 1. 执行当前评论处理阶段
	if session.Engine().Dialect().URI().DBType == schemas.MYSQL {
		return session.ForUpdate()
	}
	return session
}

// ProvideTransactionClient 提供评论仓储需要的事务能力。
func ProvideTransactionClient(client clients.MysqlClient) transactionClient {
	// 1. 执行当前评论处理阶段
	transaction, ok := client.(transactionClient)
	if !ok {
		panic("评论仓储要求 MySQL 客户端支持事务")
	}
	return transaction
}
