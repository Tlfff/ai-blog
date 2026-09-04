package service

import (
	"errors"

	articleapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/article"
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"codeup.aliyun.com/qimao/leo/lib/errassets"
	"github.com/gin-gonic/gin"
)

const (
	codeArticleImageNotFound  = 44050101 // codeArticleImageNotFound 表示正文图片不存在。
	codeArticleImageBound     = 44050102 // codeArticleImageBound 表示正文图片已归属其他文章。
	codeArticleNotFound       = 44050103 // codeArticleNotFound 表示文章不存在。
	codeArticleNotOwned       = 44050106 // codeArticleNotOwned 表示当前用户不是文章作者。
	codeArticleDuplicate      = 44050104 // codeArticleDuplicate 表示两秒内重复创建文章。
	codeInvalidImageExtension = 44050105 // codeInvalidImageExtension 表示正文图片扩展名不在白名单。
	codeArticleInvalidStatus  = 44010102 // codeArticleInvalidStatus 表示文章状态请求参数不合法。
	codeArticleDeleted        = 44050107 // codeArticleDeleted 表示已删除文章不能更新或发布。
	codeArticleNotPublished   = 44050108 // codeArticleNotPublished 表示文章尚未发表，不能公开读取。
)

// ArticleService 将文章 HTTP 协议转换为文章领域调用。
type ArticleService struct {
	useCase        article.UseCase             // useCase 是文章上下文公开业务接口。
	storage        article.Storage             // storage 用于把图片对象键转换为公开地址。
	regionResolver userdomain.IPRegionResolver // regionResolver 将作者 IP 转换为地区文案。
}

// NewArticleServer 创建文章 HTTP 服务。
func NewArticleServer(useCase article.UseCase, storage article.Storage, regionResolver userdomain.IPRegionResolver) articleapi.ArticleServiceHTTPServerController {
	// 1. 启动阶段拒绝缺少文章用例、对象存储或地区解析器
	if useCase == nil || storage == nil || regionResolver == nil {
		panic("文章 HTTP 服务缺少必要依赖")
	}
	return &ArticleService{useCase: useCase, storage: storage, regionResolver: regionResolver}
}

// GetImageUploadURL 获取正文图片的 MinIO 直传凭证。
func (s *ArticleService) GetImageUploadURL(ctx *gin.Context, request *articleapi.GetImageUploadURLRequest) (*articleapi.ImageUploadURLReply, error) {
	// 1. 读取管理员认证中间件注入的当前用户
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}

	// 2. 调用文章领域创建未绑定图片和十分钟预签名地址
	result, err := s.useCase.UploadImage(ctx.Request.Context(), currentUser.ID, request.GetFileExt())
	if err != nil {
		return nil, articleHTTPError(err)
	}

	// 3. 返回图片标识、直传地址和公开预览地址
	return &articleapi.ImageUploadURLReply{ImageId: result.ImageID, UploadUrl: result.UploadURL, Url: result.URL}, nil
}

// CreateArticle 创建文章并原子绑定正文引用图片。
func (s *ArticleService) CreateArticle(ctx *gin.Context, request *articleapi.CreateArticleRequest) (*articleapi.EmptyReply, error) {
	// 1. 读取管理员认证中间件注入的当前用户
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}

	// 2. 领域服务统一执行 Redis 防重、状态校验和事务创建
	err := s.useCase.Create(ctx.Request.Context(), article.CreateCommand{
		AuthorID: currentUser.ID, Title: request.GetTitle(), Content: request.GetContent(),
		Tags: request.GetTags(), Status: int8(request.GetStatus()),
	})
	if err != nil {
		return nil, articleHTTPError(err)
	}

	// 3. 明确标记兼容成功消息和 data=null 返回契约
	httpresponse.SetSuccess(ctx, "文章创建成功", true)
	return &articleapi.EmptyReply{}, nil
}

// GetMyArticleDetail 获取当前管理员作者可编辑的文章详情。
func (s *ArticleService) GetMyArticleDetail(ctx *gin.Context, request *articleapi.GetMyArticleDetailRequest) (*articleapi.ArticleDetailReply, error) {
	// 1. 读取当前用户，用于查询文章点赞状态
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}

	// 2. 查询文章、作者、图片映射和当前互动数据
	detail, err := s.useCase.Detail(ctx.Request.Context(), request.GetId(), currentUser.ID)
	if err != nil {
		return nil, articleHTTPError(err)
	}

	// 3. 将领域详情转换为 Proto 响应
	return s.detailReply(detail), nil
}

// UpdateArticle 更新当前管理员作者的非删除文章和正文图片关系。
func (s *ArticleService) UpdateArticle(ctx *gin.Context, request *articleapi.UpdateArticleRequest) (*articleapi.EmptyReply, error) {
	// 1. 读取管理员认证中间件注入的当前用户
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}

	// 2. 将协议请求转换为文章更新命令
	err := s.useCase.Update(ctx.Request.Context(), article.UpdateCommand{
		ArticleID: request.GetId(), AuthorID: currentUser.ID, Title: request.GetTitle(), Content: request.GetContent(),
		Tags: request.GetTags(), Status: int8(request.GetStatus()),
	})
	if err != nil {
		return nil, articleHTTPError(err)
	}

	// 3. 保持更新接口 data=null 的成功契约
	httpresponse.SetSuccess(ctx, "文章修改成功", true)
	return &articleapi.EmptyReply{}, nil
}

// PublishArticle 发布当前管理员作者的非删除文章。
func (s *ArticleService) PublishArticle(ctx *gin.Context, request *articleapi.ArticleIDRequest) (*articleapi.EmptyReply, error) {
	// 1. 读取管理员认证中间件注入的当前用户
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}

	// 2. 调用领域服务校验作者和删除状态后发布
	if err := s.useCase.Publish(ctx.Request.Context(), request.GetId(), currentUser.ID); err != nil {
		return nil, articleHTTPError(err)
	}

	// 3. 保持发布接口 data=null 的成功契约
	httpresponse.SetSuccess(ctx, "文章发布成功", true)
	return &articleapi.EmptyReply{}, nil
}

// GetArticleDetail 获取游客或登录用户可访问的已发表文章详情。
func (s *ArticleService) GetArticleDetail(ctx *gin.Context, request *articleapi.ArticleIDRequest) (*articleapi.ArticleDetailReply, error) {
	// 1. 可选读取认证身份，游客使用零值用户标识
	currentUser, _ := identity.FromContext(ctx)

	// 2. 查询已发表文章并补充当前用户点赞状态
	detail, err := s.useCase.PublicDetail(ctx.Request.Context(), request.GetId(), currentUser.ID)
	if err != nil {
		return nil, articleHTTPError(err)
	}

	// 3. 复用后台详情的稳定响应字段和图片 URL 转换
	return s.detailReply(detail), nil
}

// detailReply 将文章领域详情转换为协议响应。
func (s *ArticleService) detailReply(detail *entity.Detail) *articleapi.ArticleDetailReply {
	// 1. 转换文章、作者、时间和互动字段
	reply := &articleapi.ArticleDetailReply{
		Id: detail.Article.ID, Title: detail.Article.Title, Content: detail.Article.Content,
		Tags: detail.Article.Tags, Status: int32(detail.Article.Status), AuthorNick: detail.AuthorNickname,
		AuthorAvatar: detail.AuthorAvatar, Ip: s.regionResolver.Resolve(detail.AuthorIP), CreatedTime: detail.Article.CreatedTime.Unix(),
		UpdatedTime: detail.Article.UpdatedTime.Unix(), IsLiked: detail.IsLiked, LikeCount: detail.Article.LikeCount,
	}

	// 2. 将稳定对象键转换为当前公开图片地址
	for _, image := range detail.Images {
		reply.Images = append(reply.Images, &articleapi.ArticleImage{Id: image.ID, Url: s.storage.PublicURL(image.ObjectKey)})
	}
	return reply
}

// articleHTTPError 将文章领域错误转换为稳定业务码。
func articleHTTPError(err error) error {
	// 1. 仅映射可预期的文章业务错误，未知错误保留原始错误链
	switch {
	case errors.Is(err, article.ErrImageNotFound):
		return errassets.NewError(codeArticleImageNotFound, err.Error())
	case errors.Is(err, article.ErrImageAlreadyBound):
		return errassets.NewError(codeArticleImageBound, err.Error())
	case errors.Is(err, article.ErrArticleNotFound):
		return errassets.NewError(codeArticleNotFound, err.Error())
	case errors.Is(err, article.ErrArticleNotOwned):
		return errassets.NewError(codeArticleNotOwned, err.Error())
	case errors.Is(err, article.ErrDuplicateSubmission):
		return errassets.NewError(codeArticleDuplicate, err.Error())
	case errors.Is(err, article.ErrInvalidImageExtension):
		return errassets.NewError(codeInvalidImageExtension, err.Error())
	case errors.Is(err, article.ErrInvalidStatus):
		return errassets.NewError(codeArticleInvalidStatus, err.Error())
	case errors.Is(err, article.ErrArticleDeleted):
		return errassets.NewError(codeArticleDeleted, err.Error())
	case errors.Is(err, article.ErrArticleNotPublished):
		return errassets.NewError(codeArticleNotPublished, err.Error())
	default:
		return err
	}
}
