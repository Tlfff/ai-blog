package service

import (
	"context"
	"net/http"
	"testing"

	commentapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	commententity "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"github.com/gin-gonic/gin"
)

// commentUseCaseFake 记录评论删除领域命令。
type commentUseCaseFake struct {
	deleteCommand comment.DeleteCommand // deleteCommand 是收到的删除命令。
}

// Create 模拟评论创建。
func (*commentUseCaseFake) Create(context.Context, comment.CreateCommand) (*commententity.Comment, error) {
	// 1. 当前测试不使用创建能力
	return nil, nil
}

// Delete 记录评论删除命令。
func (f *commentUseCaseFake) Delete(_ context.Context, command comment.DeleteCommand) error {
	// 1. 保存操作者及管理员权限
	f.deleteCommand = command
	return nil
}

// ListRoots 模拟主评论列表。
func (*commentUseCaseFake) ListRoots(context.Context, comment.RootListQuery) (*comment.ListResult, error) {
	// 1. 当前测试不使用列表能力
	return &comment.ListResult{}, nil
}

// ListReplies 模拟回复列表。
func (*commentUseCaseFake) ListReplies(context.Context, uint64, comment.PageQuery) (*comment.ListResult, error) {
	// 1. 当前测试不使用列表能力
	return &comment.ListResult{}, nil
}

// commentRegionFake 返回空地区。
type commentRegionFake struct{}

// Resolve 模拟 IP 地区解析。
func (commentRegionFake) Resolve(string) string {
	// 1. 当前测试无需地区文案
	return ""
}

// TestCommentDeleteEndpointsPassOwnershipPolicy 验证普通和管理员入口传递不同所有权策略。
func TestCommentDeleteEndpointsPassOwnershipPolicy(t *testing.T) {
	// 1. 普通入口只能以当前用户身份删除
	useCase := &commentUseCaseFake{}
	server := NewCommentServer(useCase, commentRegionFake{}).(*CommentService)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/auth/comment/delete", nil)
	identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 1})
	if _, err := server.DeleteComment(ctx, &commentapi.DeleteCommentRequest{Id: 9}); err != nil {
		t.Fatal(err)
	}
	if useCase.deleteCommand.CommentID != 9 || useCase.deleteCommand.ActorID != 7 || useCase.deleteCommand.IsAdmin {
		t.Fatalf("normal command = %#v", useCase.deleteCommand)
	}

	// 2. 普通用户不能通过管理员入口获得所有权豁免
	if _, err := server.AdminDeleteComment(ctx, &commentapi.DeleteCommentRequest{Id: 9}); err == nil {
		t.Fatal("普通用户调用管理员删除入口未被拒绝")
	}
	if useCase.deleteCommand.IsAdmin {
		t.Fatalf("普通用户获得管理员删除标识: %#v", useCase.deleteCommand)
	}

	// 3. 管理员入口仅增加所有权豁免标识
	identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 8, Role: 2})
	if _, err := server.AdminDeleteComment(ctx, &commentapi.DeleteCommentRequest{Id: 9}); err != nil {
		t.Fatal(err)
	}
	if useCase.deleteCommand.CommentID != 9 || useCase.deleteCommand.ActorID != 8 || !useCase.deleteCommand.IsAdmin {
		t.Fatalf("admin command = %#v", useCase.deleteCommand)
	}
}
