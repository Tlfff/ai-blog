package service

import (
	"errors"
	"time"

	commentapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
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
)

// CommentService 将评论 HTTP 协议转换为评论领域调用。
type CommentService struct {
	useCase        comment.UseCase
	regionResolver userdomain.IPRegionResolver
}

// NewCommentServer 创建评论 HTTP 服务。
func NewCommentServer(useCase comment.UseCase, regionResolver userdomain.IPRegionResolver) commentapi.CommentServiceHTTPServerController {
	// 1. 执行当前评论处理阶段
	if useCase == nil || regionResolver == nil {
		panic("评论 HTTP 服务缺少领域服务或 IP 地区解析器")
	}
	return &CommentService{useCase: useCase, regionResolver: regionResolver}
}

// CreateComment 创建主评论或楼中楼回复。
func (s *CommentService) CreateComment(ctx *gin.Context, request *commentapi.CreateCommentRequest) (*commentapi.CreateCommentReply, error) {
	// 1. 执行当前评论处理阶段
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

// ListRootComments 查询文章主评论列表。
func (s *CommentService) ListRootComments(ctx *gin.Context, request *commentapi.RootCommentListRequest) (*commentapi.CommentListReply, error) {
	// 1. 执行当前评论处理阶段
	result, err := s.useCase.ListRoots(ctx.Request.Context(), comment.RootListQuery{ArticleID: request.GetArticleId(), AuthorID: request.GetAuthorId(), PageQuery: comment.PageQuery{LastID: request.GetLastId(), Page: request.GetPage(), PageSize: request.GetPageSize(), IsDesc: request.GetIsDesc()}})
	if err != nil {
		return nil, commentHTTPError(err)
	}
	return s.commentListReply(result), nil
}

// ListReplies 查询指定根评论的楼中楼回复。
func (s *CommentService) ListReplies(ctx *gin.Context, request *commentapi.ReplyListRequest) (*commentapi.CommentListReply, error) {
	// 1. 执行当前评论处理阶段
	result, err := s.useCase.ListReplies(ctx.Request.Context(), request.GetRootId(), comment.PageQuery{LastID: request.GetLastId(), Page: request.GetPage(), PageSize: request.GetPageSize()})
	if err != nil {
		return nil, commentHTTPError(err)
	}
	return s.commentListReply(result), nil
}

// commentListReply 执行评论上下文对应的处理。
func (s *CommentService) commentListReply(result *comment.ListResult) *commentapi.CommentListReply {
	// 1. 执行当前评论处理阶段
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
	// 1. 执行当前评论处理阶段
	if value == nil {
		return nil
	}
	return &commentapi.PublicUser{UserId: value.ID, Username: value.Nickname, Avatar: value.Avatar}
}

// commentUnixSeconds 执行评论上下文对应的处理。
func commentUnixSeconds(value time.Time) int64 {
	// 1. 执行当前评论处理阶段
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

// commentHTTPError 执行评论上下文对应的处理。
func commentHTTPError(err error) error {
	// 1. 执行当前评论处理阶段
	switch {
	case errors.Is(err, comment.ErrArticleNotPublished):
		return errassets.NewError(codeCommentArticleInvalid, "文章未发表")
	case errors.Is(err, comment.ErrRootNotFound), errors.Is(err, comment.ErrRootDeleted):
		return errassets.NewError(codeCommentRootInvalid, "根评论不存在或已删除")
	case errors.Is(err, comment.ErrDuplicateSubmission):
		return errassets.NewError(codeCommentDuplicate, "请勿重复提交评论")
	case errors.Is(err, comment.ErrInvalidReplyTarget):
		return errassets.NewError(codeCommentRootInvalid, "回复目标不属于当前评论楼")
	case errors.Is(err, comment.ErrInvalidInput):
		return errassets.NewError(codeCommentInvalid, "评论参数不合法")
	default:
		return err
	}
}
