package article

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
)

// Repository 定义文章和正文图片所需的数据访问能力。
type Repository interface {
	// CreatePendingImage 创建尚未归属文章的正文图片记录。
	CreatePendingImage(context.Context, *entity.Image) error
	// DeletePendingImage 删除预签名生成失败后仍未绑定的图片记录。
	DeletePendingImage(context.Context, uint64) error
	// CreateArticle 在同一事务中创建文章并绑定全部正文图片。
	CreateArticle(context.Context, *entity.Article, []uint64) error
	// FindDetail 查询非删除文章、作者快照和正文图片映射。
	FindDetail(context.Context, uint64, uint64) (*entity.Detail, error)
}

// Storage 定义正文图片直传和公开访问所需的对象存储能力。
type Storage interface {
	// PresignPut 生成指定有效期的 MinIO PUT 预签名地址。
	PresignPut(context.Context, string, time.Duration) (string, error)
	// PublicURL 根据稳定对象键生成公开访问地址。
	PublicURL(string) string
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
