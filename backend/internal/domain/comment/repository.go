package comment

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
)

const (
	StatusDeleted   int8 = 0
	StatusNormal    int8 = 1
	StatusPublished int8 = 3
)

// ArticleReader 定义评论创建所需的文章有效性查询契约。
type ArticleReader interface {
	// IsPublished 查询文章是否存在且处于已发表状态。
	IsPublished(context.Context, uint64) (bool, error)
}

// UserReader 定义评论列表所需的用户公开资料查询契约。
type UserReader interface {
	// FindPublic 批量查询正常用户的公开资料。
	FindPublic(context.Context, []uint64) (map[uint64]*entity.PublicUser, error)
}

// SubmissionGuard 定义评论创建的短期防重复能力。
type SubmissionGuard interface {
	// Acquire 原子占用提交指纹。
	Acquire(context.Context, string, time.Duration) (bool, error)
}

// CreateCommand 表示创建主评论或回复的领域输入。
type CreateCommand struct {
	ArticleID     uint64    // ArticleID 是有效文章标识。
	UserID        uint64    // UserID 是当前登录用户标识。
	RootID        uint64    // RootID 为0表示主评论，否则表示目标根评论。
	ReplyToUserID uint64    // ReplyToUserID 是被回复用户标识。
	Content       string    // Content 是评论正文。
	IP            string    // IP 是评论创建来源地址。
	CreatedTime   time.Time // CreatedTime 是评论创建时间。
}

// PageQuery 表示评论列表的规范化分页输入。
type PageQuery struct {
	LastID   uint64 // LastID 大于0时启用游标分页。
	Page     uint64 // Page 是 Offset 分页页码。
	PageSize uint64 // PageSize 是每页数量。
	IsDesc   bool   // IsDesc 表示是否按 ID 倒序。
}

// RootListQuery 表示主评论列表查询条件。
type RootListQuery struct {
	ArticleID uint64 // ArticleID 是文章标识。
	AuthorID  uint64 // AuthorID 大于0时只查询楼主评论。
	PageQuery        // PageQuery 是通用分页条件。
}

// ListResult 表示评论列表及分页元数据。
type ListResult struct {
	Items    []*entity.Item // Items 是当前页评论。
	LastID   uint64         // LastID 是当前页末条评论标识。
	Total    uint64         // Total 是符合条件的评论总数。
	Page     uint64         // Page 是规范化后的页码。
	PageSize uint64         // PageSize 是规范化后的每页数量。
}

// Repository 定义评论上下文的数据访问能力。
type Repository interface {
	// Create 在事务中创建评论并维护根评论回复数。
	Create(context.Context, *entity.Comment) error
	// FindRoot 查询正常或已删除状态的根评论，用于回复前校验。
	FindRoot(context.Context, uint64) (*entity.Comment, error)
	// HasReplyTarget 查询用户是否属于指定根评论及其正常直属回复链。
	HasReplyTarget(context.Context, uint64, uint64) (bool, error)
	// ListRoots 分页查询正常主评论。
	ListRoots(context.Context, RootListQuery) (*ListResult, error)
	// ListReplies 分页查询指定根评论的正常回复。
	ListReplies(context.Context, uint64, PageQuery) (*ListResult, error)
}
