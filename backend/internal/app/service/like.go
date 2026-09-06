package service

import (
	"errors"

	likeapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"codeup.aliyun.com/qimao/leo/lib/errassets"
	"github.com/gin-gonic/gin"
)

const (
	codeLikeInvalid          = 44010104
	codeLikeNotAuthenticated = 44030101
	codeLikeArticleInvalid   = 44050116
)

// LikeService 将文章点赞 HTTP 协议转换为点赞领域调用。
type LikeService struct {
	useCase like.UseCase // useCase 提供文章点赞与取消点赞能力。
}

// NewLikeServer 创建文章点赞 HTTP 服务。
func NewLikeServer(useCase like.UseCase) likeapi.LikeServiceHTTPServerController {
	// 1. 启动阶段拒绝缺少点赞领域服务
	if useCase == nil {
		panic("点赞 HTTP 服务缺少领域服务")
	}
	return &LikeService{useCase: useCase}
}

// LikeArticle 幂等点赞文章。
func (s *LikeService) LikeArticle(ctx *gin.Context, request *likeapi.ArticleLikeRequest) (*likeapi.EmptyReply, error) {
	// 1. 读取当前登录用户并建立点赞事实
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeLikeNotAuthenticated, "未登录")
	}
	if err := s.useCase.LikeArticle(ctx.Request.Context(), currentUser.ID, request.GetArticleId()); err != nil {
		return nil, likeHTTPError(err)
	}

	// 2. 保持点赞接口 data=null 的兼容契约
	httpresponse.SetSuccess(ctx, "点赞成功", true)
	return &likeapi.EmptyReply{}, nil
}

// CancelArticleLike 幂等取消文章点赞。
func (s *LikeService) CancelArticleLike(ctx *gin.Context, request *likeapi.ArticleLikeRequest) (*likeapi.EmptyReply, error) {
	// 1. 读取当前登录用户并取消点赞事实
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeLikeNotAuthenticated, "未登录")
	}
	if err := s.useCase.CancelArticleLike(ctx.Request.Context(), currentUser.ID, request.GetArticleId()); err != nil {
		return nil, likeHTTPError(err)
	}

	// 2. 保持取消点赞接口 data=null 的兼容契约
	httpresponse.SetSuccess(ctx, "取消点赞成功", true)
	return &likeapi.EmptyReply{}, nil
}

// likeHTTPError 将点赞领域错误转换为稳定业务码。
func likeHTTPError(err error) error {
	// 1. 仅映射可预期业务错误，未知依赖错误保留原始错误链
	switch {
	case errors.Is(err, like.ErrInvalidInput):
		return errassets.NewError(codeLikeInvalid, "点赞参数不合法")
	case errors.Is(err, like.ErrArticleUnavailable):
		return errassets.NewError(codeLikeArticleInvalid, "文章不存在或未发表")
	default:
		return err
	}
}
