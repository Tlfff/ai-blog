package service

import (
	"errors"
	"time"

	commentapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"codeup.aliyun.com/qimao/leo/lib/errassets"
	"github.com/gin-gonic/gin"
)

const (
	codeCommentInvalid          = 44010103
	codeCommentArticleInvalid   = 44050112
	codeCommentRootInvalid      = 44050113
	codeCommentDuplicate        = 44050114
	codeCommentNotAuthenticated = 44030101
	codeCommentPermissionDenied = 44030102
	codeCommentNotFound         = 44050115
)

// CommentService 将评论 HTTP 协议转换为评论领域调用。
type CommentService struct {
	useCase        comment.UseCase             // useCase 提供评论创建与列表领域能力。
	regionResolver userdomain.IPRegionResolver // regionResolver 将评论 IP 转换为地区文案。
}

// NewCommentServer 创建评论 HTTP 服务。
func NewCommentServer(useCase comment.UseCase, regionResolver userdomain.IPRegionResolver) commentapi.CommentServiceHTTPServerController {
	// 1. 校验并保存评论领域服务和地区解析器
	if useCase == nil || regionResolver == nil {
		panic("评论 HTTP 服务缺少领域服务或 IP 地区解析器")
	}
	return &CommentService{useCase: useCase, regionResolver: regionResolver}
}

// CreateComment 创建主评论或楼中楼回复。
func (s *CommentService) CreateComment(ctx *gin.Context, request *commentapi.CreateCommentRequest) (*commentapi.CreateCommentReply, error) {
	// 1. 读取登录身份并创建主评论或直属回复
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeCommentNotAuthenticated, "未登录")
	}
	created, err := s.useCase.Create(ctx.Request.Context(), comment.CreateCommand{ArticleID: request.GetArticleId(), UserID: currentUser.ID, RootID: request.GetRootId(), ReplyToUserID: request.GetReplyToUserId(), Content: request.GetContent(), IP: ctx.ClientIP()})
	if err != nil {
		return nil, commentHTTPError(err)
	}
	return &commentapi.CreateCommentReply{Id: created.ID, CreatedTime: commentUnixSeconds(created.CreatedTime)}, nil
}

// DeleteComment 删除当前用户自己的评论。
func (s *CommentService) DeleteComment(ctx *gin.Context, request *commentapi.DeleteCommentRequest) (*commentapi.EmptyReply, error) {
	// 1. 读取当前用户并按所有权规则删除评论
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeCommentNotAuthenticated, "未登录")
	}
	if err := s.useCase.Delete(ctx.Request.Context(), comment.DeleteCommand{CommentID: request.GetId(), ActorID: currentUser.ID}); err != nil {
		return nil, commentHTTPError(err)
	}

	// 2. 保持删除接口 data=null 的兼容契约
	httpresponse.SetSuccess(ctx, "评论删除成功", true)
	return &commentapi.EmptyReply{}, nil
}

// AdminDeleteComment 允许管理员绕过评论所有权校验。
func (s *CommentService) AdminDeleteComment(ctx *gin.Context, request *commentapi.DeleteCommentRequest) (*commentapi.EmptyReply, error) {
	// 1. 再次校验管理员身份，避免应用服务被非 HTTP 入口误用
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeCommentNotAuthenticated, "未登录")
	}
	if currentUser.Role != userdomain.RoleAdmin {
		return nil, errassets.NewError(codeCommentPermissionDenied, "无管理员权限")
	}
	if err := s.useCase.Delete(ctx.Request.Context(), comment.DeleteCommand{CommentID: request.GetId(), ActorID: currentUser.ID, IsAdmin: true}); err != nil {
		return nil, commentHTTPError(err)
	}

	// 2. 管理员入口复用相同删除结果契约
	httpresponse.SetSuccess(ctx, "评论删除成功", true)
	return &commentapi.EmptyReply{}, nil
}

// ListRootComments 查询文章主评论列表。
func (s *CommentService) ListRootComments(ctx *gin.Context, request *commentapi.RootCommentListRequest) (*commentapi.CommentListReply, error) {
	// 1. 转换主评论筛选参数并查询公开列表
	result, err := s.useCase.ListRoots(ctx.Request.Context(), comment.RootListQuery{ArticleID: request.GetArticleId(), AuthorID: request.GetAuthorId(), PageQuery: comment.PageQuery{LastID: request.GetLastId(), Page: request.GetPage(), PageSize: request.GetPageSize(), IsDesc: request.GetIsDesc()}})
	if err != nil {
		return nil, commentHTTPError(err)
	}
	return s.commentListReply(result), nil
}

// ListReplies 查询指定根评论的楼中楼回复。
func (s *CommentService) ListReplies(ctx *gin.Context, request *commentapi.ReplyListRequest) (*commentapi.CommentListReply, error) {
	// 1. 转换回复分页参数并查询直属回复
	result, err := s.useCase.ListReplies(ctx.Request.Context(), request.GetRootId(), comment.PageQuery{LastID: request.GetLastId(), Page: request.GetPage(), PageSize: request.GetPageSize()})
	if err != nil {
		return nil, commentHTTPError(err)
	}
	return s.commentListReply(result), nil
}

// commentListReply 执行评论上下文对应的处理。
func (s *CommentService) commentListReply(result *comment.ListResult) *commentapi.CommentListReply {
	// 1. 转换评论、用户、地区和分页字段
	items := make([]*commentapi.CommentItem, 0, len(result.Items))
	for _, item := range result.Items {
		user := publicUserReply(item.User)
		replyTo := publicUserReply(item.ReplyToUser)
		items = append(items, &commentapi.CommentItem{Id: item.Comment.ID, ArticleId: item.Comment.ArticleID, RootId: item.Comment.RootID, User: user, ReplyToUser: replyTo, Content: item.Comment.Content, ReplyCount: uint64(item.Comment.ReplyCount), Ip: s.regionResolver.Resolve(item.Comment.IP), CreatedTime: commentUnixSeconds(item.Comment.CreatedTime), Status: int32(item.Comment.Status), LikeCount: item.Comment.LikeCount})
	}
	return &commentapi.CommentListReply{List: items, LastId: result.LastID, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}

// publicUserReply 执行评论上下文对应的处理。
func publicUserReply(value *entity.PublicUser) *commentapi.PublicUser {
	// 1. 将公开用户领域数据转换为协议对象
	if value == nil {
		return nil
	}
	return &commentapi.PublicUser{UserId: value.ID, Username: value.Nickname, Avatar: value.Avatar}
}

// commentUnixSeconds 执行评论上下文对应的处理。
func commentUnixSeconds(value time.Time) int64 {
	// 1. 将评论时间转换为 Unix 秒
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

// commentHTTPError 执行评论上下文对应的处理。
func commentHTTPError(err error) error {
	// 1. 将评论领域错误映射为稳定业务码
	switch {
	case errors.Is(err, comment.ErrArticleNotPublished):
		return errassets.NewError(codeCommentArticleInvalid, "文章未发表")
	case errors.Is(err, comment.ErrRootNotFound), errors.Is(err, comment.ErrRootDeleted):
		return errassets.NewError(codeCommentRootInvalid, "根评论不存在或已删除")
	case errors.Is(err, comment.ErrDuplicateSubmission):
		return errassets.NewError(codeCommentDuplicate, "请勿重复提交评论")
	case errors.Is(err, comment.ErrInvalidReplyTarget):
		return errassets.NewError(codeCommentRootInvalid, "回复目标不属于当前评论楼")
	case errors.Is(err, comment.ErrCommentNotFound):
		return errassets.NewError(codeCommentNotFound, "评论不存在")
	case errors.Is(err, comment.ErrCommentPermissionDenied):
		return errassets.NewError(codeCommentPermissionDenied, "无权删除该评论")
	case errors.Is(err, comment.ErrInvalidInput):
		return errassets.NewError(codeCommentInvalid, "评论参数不合法")
	default:
		return err
	}
}
