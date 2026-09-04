package article

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const (
	StatusDeleted   int8 = 1 // StatusDeleted 表示文章已移入垃圾箱。
	StatusDraft     int8 = 2 // StatusDraft 表示文章为草稿。
	StatusPublished int8 = 3 // StatusPublished 表示文章已发表。

	uploadURLTTL       = 10 * time.Minute // uploadURLTTL 是正文图片预签名地址有效期。
	submissionGuardTTL = 2 * time.Second  // submissionGuardTTL 是创建文章防重复窗口。
)

// CreateCommand 表示创建文章所需的领域输入。
type CreateCommand struct {
	AuthorID uint64   // AuthorID 是当前管理员作者标识。
	Title    string   // Title 是文章标题。
	Content  string   // Content 是包含稳定图片引用的 Markdown 正文。
	Tags     []string // Tags 是文章标签集合。
	Status   int8     // Status 是目标状态：0-兼容状态，2-草稿，3-已发表。
}

// UpdateCommand 表示更新文章及正文图片关系所需的领域输入。
type UpdateCommand struct {
	ArticleID uint64   // ArticleID 是待更新文章唯一标识。
	AuthorID  uint64   // AuthorID 是当前管理员作者标识。
	Title     string   // Title 是更新后的文章标题。
	Content   string   // Content 是包含稳定图片引用的 Markdown 正文。
	Tags      []string // Tags 是更新后的文章标签集合。
	Status    int8     // Status 是目标状态：0-兼容状态，2-草稿，3-已发表。
}

// UploadResult 表示正文图片直传凭证和稳定引用信息。
type UploadResult struct {
	ImageID   uint64 // ImageID 是前端写入 image:// 引用的图片标识。
	UploadURL string // UploadURL 是 10 分钟有效的 MinIO PUT 预签名地址。
	URL       string // URL 是图片上传完成后的公开预览地址。
}

// UseCase 定义文章上下文向应用层暴露的业务能力。
type UseCase interface {
	// UploadImage 创建未绑定图片记录并返回 MinIO 直传凭证。
	UploadImage(context.Context, uint64, string) (*UploadResult, error)
	// Create 创建文章并原子绑定正文引用图片。
	Create(context.Context, CreateCommand) error
	// Detail 查询后台可编辑文章详情。
	Detail(context.Context, uint64, uint64) (*entity.Detail, error)
	// Update 更新作者自己的非删除文章并同步正文图片关系。
	Update(context.Context, UpdateCommand) error
	// Publish 发布作者自己的非删除文章。
	Publish(context.Context, uint64, uint64) error
	// PublicDetail 查询已发表文章详情和当前用户点赞状态。
	PublicDetail(context.Context, uint64, uint64) (*entity.Detail, error)
}

// Service 实现文章创建、更新、发布、图片上传和详情规则。
type Service struct {
	repository Repository             // repository 提供文章和正文图片事务持久化能力。
	storage    Storage                // storage 提供 MinIO 预签名和公开地址能力。
	likes      LikeReader             // likes 提供点赞上下文的稳定查询能力。
	guard      SubmissionGuard        // guard 提供 Redis 跨实例防重复提交能力。
	allowed    AllowedImageExtensions // allowed 是配置允许的正文图片扩展名集合。
	now        func() time.Time       // now 提供可测试的当前时间。
}

// NewService 创建文章领域服务。
//
// 参数说明：
//   - repository：文章和正文图片仓储。
//   - storage：正文图片对象存储。
//   - likes：文章点赞状态查询器。
//   - guard：文章创建防重复提交仓储。
//   - allowed：允许上传的正文图片扩展名集合，不能为空。
func NewService(repository Repository, storage Storage, likes LikeReader, guard SubmissionGuard, allowed AllowedImageExtensions) *Service {
	// 1. 启动阶段拒绝缺少文章业务必要依赖
	if repository == nil || storage == nil || likes == nil || guard == nil || len(allowed) == 0 {
		panic("文章领域服务缺少必要依赖")
	}
	return &Service{repository: repository, storage: storage, likes: likes, guard: guard, allowed: allowed, now: time.Now}
}

// UploadImage 校验扩展名并创建正文图片直传凭证。
func (s *Service) UploadImage(ctx context.Context, _ uint64, extension string) (*UploadResult, error) {
	// 1. 规范化扩展名并按配置白名单拒绝非图片文件
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	if _, allowed := s.allowed[extension]; !allowed {
		return nil, ErrInvalidImageExtension
	}

	// 2. 创建按年月组织且不会随预签名地址变化的对象键
	now := s.now()
	objectKey := fmt.Sprintf("article/%s/%s.%s", now.Format("200601"), uuid.NewString(), extension)
	image := &entity.Image{ObjectKey: objectKey}
	if err := s.repository.CreatePendingImage(ctx, image); err != nil {
		return nil, fmt.Errorf("创建未绑定正文图片: %w", err)
	}

	// 3. 使用固定十分钟有效期生成 MinIO PUT 预签名地址
	uploadURL, err := s.storage.PresignPut(ctx, objectKey, uploadURLTTL)
	if err != nil {
		// 3.1 预签名失败时删除刚创建的未绑定记录，避免产生孤立图片
		if deleteErr := s.repository.DeletePendingImage(ctx, image.ID); deleteErr != nil {
			return nil, fmt.Errorf("生成正文图片预签名地址: %w；清理图片记录: %v", err, deleteErr)
		}
		return nil, fmt.Errorf("生成正文图片预签名地址: %w", err)
	}
	return &UploadResult{ImageID: image.ID, UploadURL: uploadURL, URL: s.storage.PublicURL(objectKey)}, nil
}

// Create 校验文章状态、防重复提交并原子创建文章。
func (s *Service) Create(ctx context.Context, command CreateCommand) error {
	// 1. 保留状态 0 兼容行为，只允许新增草稿或已发表文章
	if command.Status != 0 && command.Status != StatusDraft && command.Status != StatusPublished {
		return ErrInvalidStatus
	}

	// 2. 使用 Redis 原子占位保证所有实例共享两秒防重复窗口
	acquired, err := s.guard.Acquire(ctx, submissionFingerprint(command), submissionGuardTTL)
	if err != nil {
		return fmt.Errorf("占用文章创建防重键: %w", err)
	}
	if !acquired {
		return ErrDuplicateSubmission
	}

	// 3. 解析稳定图片引用，由仓储在同一事务中创建文章并完成绑定
	now := s.now()
	article := &entity.Article{
		AuthorID: command.AuthorID, Title: command.Title, Content: command.Content,
		Tags: append([]string(nil), command.Tags...), Status: command.Status,
		CreatedTime: now, UpdatedTime: now,
	}
	return s.repository.CreateArticle(ctx, article, referencedImageIDs(command.Content))
}

// Detail 查询后台文章详情并补充当前用户点赞状态。
func (s *Service) Detail(ctx context.Context, articleID, userID uint64) (*entity.Detail, error) {
	// 1. 查询文章上下文拥有的详情数据
	detail, err := s.repository.FindDetail(ctx, articleID, userID)
	if err != nil {
		return nil, err
	}
	if detail == nil || detail.Article == nil {
		return nil, ErrArticleNotFound
	}

	// 2. 通过点赞上下文稳定查询契约补充当前用户点赞事实
	detail.IsLiked, err = s.likes.IsArticleLiked(ctx, userID, articleID)
	if err != nil {
		return nil, fmt.Errorf("查询当前用户文章点赞状态: %w", err)
	}
	return detail, nil
}

// Update 校验目标状态并原子更新文章和正文图片关系。
func (s *Service) Update(ctx context.Context, command UpdateCommand) error {
	// 1. 保留状态 0 兼容行为，只允许更新为草稿或已发表状态
	if command.Status != 0 && command.Status != StatusDraft && command.Status != StatusPublished {
		return ErrInvalidStatus
	}

	// 2. 在仓储锁定文章后执行作者、删除状态和字段更新规则
	return s.repository.UpdateArticle(ctx, command.ArticleID, referencedImageIDs(command.Content), func(article *entity.Article) error {
		if err := authorizeArticleMutation(article, command.AuthorID); err != nil {
			return err
		}
		article.Title = command.Title
		article.Content = command.Content
		article.Tags = append([]string(nil), command.Tags...)
		article.Status = command.Status
		article.UpdatedTime = s.now()
		return nil
	})
}

// Publish 将作者自己的非删除文章更新为已发表状态。
func (s *Service) Publish(ctx context.Context, articleID, authorID uint64) error {
	// 1. 在仓储锁定文章后执行作者、删除状态和发布规则
	return s.repository.PublishArticle(ctx, articleID, func(article *entity.Article) error {
		if err := authorizeArticleMutation(article, authorID); err != nil {
			return err
		}
		article.Status = StatusPublished
		article.UpdatedTime = s.now()
		return nil
	})
}

// authorizeArticleMutation 校验文章作者和可修改状态。
func authorizeArticleMutation(article *entity.Article, authorID uint64) error {
	// 1. 只有文章作者可以修改内容或发布
	if article.AuthorID != authorID {
		return ErrArticleNotOwned
	}

	// 2. 已移入垃圾箱的文章不能通过更新或发布接口修改
	if article.Status == StatusDeleted {
		return ErrArticleDeleted
	}
	return nil
}

// PublicDetail 查询已发表文章详情并补充当前用户点赞状态。
func (s *Service) PublicDetail(ctx context.Context, articleID, userID uint64) (*entity.Detail, error) {
	// 1. 查询仅允许公开读取的已发表文章详情
	detail, err := s.repository.FindPublicDetail(ctx, articleID)
	if err != nil {
		return nil, err
	}
	if detail == nil || detail.Article == nil {
		return nil, ErrArticleNotFound
	}

	// 2. 游客由点赞查询契约固定返回未点赞，登录用户查询实际状态
	detail.IsLiked, err = s.likes.IsArticleLiked(ctx, userID, articleID)
	if err != nil {
		return nil, fmt.Errorf("查询当前用户文章点赞状态: %w", err)
	}
	return detail, nil
}

// referencedImageIDs 解析并去重正文中的稳定图片引用。
func referencedImageIDs(content string) []uint64 {
	// 1. 解析 Markdown AST，避免普通文本和代码块中的占位符被误绑定
	document := goldmark.DefaultParser().Parse(text.NewReader([]byte(content)))
	seen := make(map[uint64]struct{})
	imageIDs := make([]uint64, 0)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		image, ok := node.(*ast.Image)
		if !ok {
			return ast.WalkContinue, nil
		}
		imageID, ok := stableImageID(string(image.Destination))
		if !ok {
			return ast.WalkContinue, nil
		}
		if _, exists := seen[imageID]; exists {
			return ast.WalkContinue, nil
		}
		seen[imageID] = struct{}{}
		imageIDs = append(imageIDs, imageID)
		return ast.WalkContinue, nil
	})
	return imageIDs
}

// stableImageID 解析完整且有效的 image:// 稳定图片引用。
func stableImageID(source string) (uint64, bool) {
	// 1. 只接受协议后完整内容为正整数的系统图片地址
	const prefix = "image://"
	if !strings.HasPrefix(source, prefix) {
		return 0, false
	}
	imageID, err := strconv.ParseUint(strings.TrimPrefix(source, prefix), 10, 64)
	return imageID, err == nil && imageID > 0
}

// submissionFingerprint 生成不包含明文正文的稳定提交指纹。
func submissionFingerprint(command CreateCommand) string {
	// 1. 复制并排序标签，避免同一标签集合因顺序差异绕过防重
	tags := append([]string(nil), command.Tags...)
	sort.Strings(tags)
	payload := fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%d", command.AuthorID, command.Title, command.Content, strings.Join(tags, "\x00"), command.Status)
	digest := sha256.Sum256([]byte(payload))
	return "article:create:" + hex.EncodeToString(digest[:])
}
