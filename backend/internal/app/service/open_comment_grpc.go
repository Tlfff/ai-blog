package service

import (
	"context"
	"errors"

	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	commentdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OpenCommentGRPCService 将开放评论协议适配到评论上下文公开查询接口。
type OpenCommentGRPCService struct {
	blogopenv1.UnimplementedCommentServiceServer                            // UnimplementedCommentServiceServer 保持生成接口的前向兼容。
	comments                                     commentdomain.QueryUseCase // comments 是评论上下文公开的只读应用接口。
}

// NewOpenCommentGRPCServer 创建开放评论统计 gRPC 服务。
func NewOpenCommentGRPCServer(comments commentdomain.QueryUseCase) blogopenv1.CommentServiceServer {
	// 1. 启动阶段拒绝缺少评论公开查询接口
	if comments == nil {
		panic("开放评论 gRPC 服务缺少查询接口")
	}
	return &OpenCommentGRPCService{comments: comments}
}

// GetCommentStats 返回评论点赞数与点赞数加回复数的热度值。
func (s *OpenCommentGRPCService) GetCommentStats(ctx context.Context, request *blogopenv1.GetCommentStatsRequest) (*blogopenv1.GetCommentStatsReply, error) {
	// 1. 在调用评论上下文前校验评论标识
	if request == nil || request.GetCommentId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "comment_id 必须大于 0")
	}

	// 2. 只通过评论上下文公开接口查询统计
	stats, err := s.comments.GetStats(ctx, request.GetCommentId())
	if err != nil {
		return nil, commentGRPCError(err)
	}
	if stats == nil {
		return nil, status.Error(codes.Internal, "查询评论统计失败")
	}
	return &blogopenv1.GetCommentStatsReply{CommentId: stats.CommentID, HotValue: stats.HotValue(), LikeCount: stats.LikeCount}, nil
}

// commentGRPCError 将评论查询错误映射为标准 gRPC Code。
func commentGRPCError(err error) error {
	// 1. 区分参数、资源不存在和内部依赖错误
	switch {
	case errors.Is(err, commentdomain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, "评论参数不合法")
	case errors.Is(err, commentdomain.ErrCommentNotFound):
		return status.Error(codes.NotFound, "评论不存在")
	default:
		return status.Error(codes.Internal, "查询评论统计失败")
	}
}
