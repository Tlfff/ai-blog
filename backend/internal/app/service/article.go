package service

import (
	"errors"

	articleapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/article"
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
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
	reply := &articleapi.ArticleDetailReply{
		Id: detail.Article.ID, Title: detail.Article.Title, Content: detail.Article.Content,
		Tags: detail.Article.Tags, Status: int32(detail.Article.Status), AuthorNick: detail.AuthorNickname,
		AuthorAvatar: detail.AuthorAvatar, Ip: s.regionResolver.Resolve(detail.AuthorIP), CreatedTime: detail.Article.CreatedTime.Unix(),
		UpdatedTime: detail.Article.UpdatedTime.Unix(), IsLiked: detail.IsLiked, LikeCount: detail.Article.LikeCount,
	}
	for _, image := range detail.Images {
		reply.Images = append(reply.Images, &articleapi.ArticleImage{Id: image.ID, Url: s.storage.PublicURL(image.ObjectKey)})
	}
	return reply, nil
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
	default:
		return err
	}
}
