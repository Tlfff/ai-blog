package article

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
)

// ArticleMutation 定义事务内对已锁定文章执行的领域规则。
type ArticleMutation func(*entity.Article) error

// ArticleClearValidation 定义事务内对文章和图片快照执行的清理校验。
type ArticleClearValidation func(*entity.Article, []*entity.Image) error

// ClearTarget 表示彻底删除前需要校验和暂存的文章资源快照。
type ClearTarget struct {
	Article *entity.Article // Article 是待彻底删除的文章。
	Images  []*entity.Image // Images 是文章当前绑定的正文图片。
}

// StagedObjectDeletion 表示可提交或回滚的对象删除操作。
type StagedObjectDeletion interface {
	// OriginalKey 返回数据库保存的原始稳定对象键。
	OriginalKey() string
	// Commit 清理用于回滚的隔离对象，完成删除。
	Commit(context.Context) error
	// Rollback 将隔离对象恢复到原始稳定对象键。
	Rollback(context.Context) error
}

// Repository 定义文章和正文图片所需的数据访问能力。
type Repository interface {
	// CreatePendingImage 创建尚未归属文章的正文图片记录。
	CreatePendingImage(context.Context, *entity.Image) error
	// DeletePendingImage 删除预签名生成失败后仍未绑定的图片记录。
	DeletePendingImage(context.Context, uint64) error
	// CreateArticle 在同一事务中创建文章并绑定全部正文图片。
	CreateArticle(context.Context, *entity.Article, []uint64) error
	// UpdateArticle 在同一事务中执行领域更新并同步正文图片关系。
	UpdateArticle(context.Context, uint64, []uint64, ArticleMutation) error
	// ChangeArticleStatus 在同一事务中执行文章状态变更领域规则。
	ChangeArticleStatus(context.Context, uint64, ArticleMutation) error
	// ListArticles 分页查询当前作者的文章。
	ListArticles(context.Context, ListQuery) (*ListResult, error)
	// FindClearTarget 查询待彻底删除的文章和绑定图片快照。
	FindClearTarget(context.Context, uint64) (*ClearTarget, error)
	// ImageExistsByObjectKey 查询数据库是否仍引用指定稳定对象键。
	ImageExistsByObjectKey(context.Context, string) (bool, error)
	// ClearArticle 在同一事务中复核快照并硬删除数据库记录。
	ClearArticle(context.Context, uint64, ArticleClearValidation) error
	// FindDetail 查询非删除文章、作者快照和正文图片映射。
	FindDetail(context.Context, uint64, uint64) (*entity.Detail, error)
	// FindPublicDetail 查询已发表文章、作者快照和正文图片映射。
	FindPublicDetail(context.Context, uint64) (*entity.Detail, error)
}

// Storage 定义正文图片直传和公开访问所需的对象存储能力。
type Storage interface {
	// PresignPut 生成指定有效期的 MinIO PUT 预签名地址。
	PresignPut(context.Context, string, time.Duration) (string, error)
	// PublicURL 根据稳定对象键生成公开访问地址。
	PublicURL(string) string
	// StageDelete 暂存并删除原始对象，返回可提交或回滚的操作。
	StageDelete(context.Context, string) (StagedObjectDeletion, error)
	// ListStagedDeletions 列出超过安全宽限期的持久化暂存删除记录。
	ListStagedDeletions(context.Context) ([]StagedObjectDeletion, error)
}

// LikeReader 定义文章上下文读取点赞事实的稳定查询契约。
type LikeReader interface {
	// IsArticleLiked 查询用户是否已点赞指定文章。
	IsArticleLiked(context.Context, uint64, uint64) (bool, error)
}

// SubmissionGuard 定义跨实例防重复提交能力。
type SubmissionGuard interface {
	// Acquire 原子占用带有效期的提交指纹，已存在时返回 false。
	Acquire(context.Context, string, time.Duration) (bool, error)
}

// AllowedImageExtensions 是允许用于正文图片的扩展名集合。
type AllowedImageExtensions map[string]struct{}
