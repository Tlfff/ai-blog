package service

import (
	"context"
	"errors"
	"testing"

	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	commentdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeOpenCommentQuery 记录开放评论 gRPC 对评论上下文接口的调用。
type fakeOpenCommentQuery struct {
	commentID uint64               // commentID 是收到的评论标识。
	stats     *commentdomain.Stats // stats 是预设评论统计。
	err       error                // err 是预设查询错误。
}

// GetStats 记录评论标识并返回预设结果。
func (f *fakeOpenCommentQuery) GetStats(_ context.Context, commentID uint64) (*commentdomain.Stats, error) {
	// 1. 证明应用服务只调用评论上下文公开查询接口
	f.commentID = commentID
	return f.stats, f.err
}

// TestOpenCommentGRPCServiceReturnsLikeAndHotCounts 验证评论点赞数和热度值转换。
func TestOpenCommentGRPCServiceReturnsLikeAndHotCounts(t *testing.T) {
	// 1. 评论上下文返回点赞4、回复3时，RPC 热度为7
	query := &fakeOpenCommentQuery{stats: &commentdomain.Stats{CommentID: 9, LikeCount: 4, ReplyCount: 3}}
	service := NewOpenCommentGRPCServer(query)
	reply, err := service.GetCommentStats(context.Background(), &blogopenv1.GetCommentStatsRequest{CommentId: 9})
	if err != nil {
		t.Fatal(err)
	}
	if query.commentID != 9 || reply.GetCommentId() != 9 || reply.GetLikeCount() != 4 || reply.GetHotValue() != 7 {
		t.Fatalf("reply = %#v, queried = %d", reply, query.commentID)
	}
}

// TestOpenCommentGRPCServiceMapsValidationAndDomainErrors 验证评论 RPC 标准错误码。
func TestOpenCommentGRPCServiceMapsValidationAndDomainErrors(t *testing.T) {
	// 1. 覆盖参数、评论不存在和内部依赖错误
	tests := []struct {
		name      string     // name 是测试场景名称。
		commentID uint64     // commentID 是请求评论标识。
		err       error      // err 是评论上下文预设错误。
		want      codes.Code // want 是期望 gRPC Code。
	}{
		{name: "无效标识", want: codes.InvalidArgument},
		{name: "评论不存在", commentID: 9, err: commentdomain.ErrCommentNotFound, want: codes.NotFound},
		{name: "内部错误", commentID: 9, err: errors.New("database secret"), want: codes.Internal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := &fakeOpenCommentQuery{err: test.err}
			service := NewOpenCommentGRPCServer(query)
			_, err := service.GetCommentStats(context.Background(), &blogopenv1.GetCommentStatsRequest{CommentId: test.commentID})
			if status.Code(err) != test.want {
				t.Fatalf("error = %v, want code %v", err, test.want)
			}
			if test.want == codes.Internal && err.Error() == "database secret" {
				t.Fatalf("internal error leaked detail: %v", err)
			}
		})
	}
}
