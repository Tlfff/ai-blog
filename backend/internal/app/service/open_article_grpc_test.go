package service

import (
	"context"
	"errors"
	"testing"
	"time"

	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	articledomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeOpenArticleQuery 记录开放文章 gRPC 对文章上下文接口的调用。
type fakeOpenArticleQuery struct {
	command articledomain.AvailableListCommand // command 是收到的 Offset 分页参数。
	result  *articledomain.AvailableListResult // result 是预设文章列表。
	err     error                              // err 是预设查询错误。
}

// ListAvailable 记录分页参数并返回预设结果。
func (f *fakeOpenArticleQuery) ListAvailable(_ context.Context, command articledomain.AvailableListCommand) (*articledomain.AvailableListResult, error) {
	// 1. 证明应用服务只调用文章上下文公开查询接口
	f.command = command
	return f.result, f.err
}

// TestOpenArticleGRPCServiceMapsPublishedArticleList 验证开放文章字段与 Offset 分页转换。
func TestOpenArticleGRPCServiceMapsPublishedArticleList(t *testing.T) {
	// 1. 准备文章上下文公开结果
	createdAt := time.Date(2026, time.September, 7, 9, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	query := &fakeOpenArticleQuery{result: &articledomain.AvailableListResult{Articles: []*entity.Article{{ID: 9, Title: "已发表", Tags: []string{"go", "grpc"}, Status: articledomain.StatusPublished, Content: "不应返回", CreatedTime: createdAt, UpdatedTime: updatedAt}}, Total: 21, Page: 2, PageSize: 10}}
	service := NewOpenArticleGRPCServer(query)

	// 2. 调用 RPC 并验证只返回开放字段
	reply, err := service.GetAvailableList(context.Background(), &blogopenv1.GetAvailableListRequest{Page: 2, PageSize: 10, IsDesc: true})
	if err != nil {
		t.Fatal(err)
	}
	if query.command.Page != 2 || query.command.PageSize != 10 || !query.command.IsDesc {
		t.Fatalf("command = %#v", query.command)
	}
	if reply.GetTotal() != 21 || reply.GetPage() != 2 || len(reply.GetItems()) != 1 {
		t.Fatalf("reply = %#v", reply)
	}
	item := reply.GetItems()[0]
	if item.GetId() != 9 || item.GetTitle() != "已发表" || len(item.GetTags()) != 2 || item.GetCreatedTime() != createdAt.Unix() || item.GetUpdatedTime() != updatedAt.Unix() {
		t.Fatalf("item = %#v", item)
	}
}

// TestOpenArticleGRPCServiceMapsValidationAndDependencyErrors 验证文章 RPC 标准错误码。
func TestOpenArticleGRPCServiceMapsValidationAndDependencyErrors(t *testing.T) {
	// 1. 无效分页在调用文章上下文前被拒绝
	query := &fakeOpenArticleQuery{}
	service := NewOpenArticleGRPCServer(query)
	if _, err := service.GetAvailableList(context.Background(), &blogopenv1.GetAvailableListRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid error = %v", err)
	}

	// 2. 内部依赖错误不泄漏原始详情
	query.err = errors.New("mysql password leaked")
	if _, err := service.GetAvailableList(context.Background(), &blogopenv1.GetAvailableListRequest{Page: 1, PageSize: 10}); status.Code(err) != codes.Internal || err.Error() == "mysql password leaked" {
		t.Fatalf("internal error = %v", err)
	}
}
