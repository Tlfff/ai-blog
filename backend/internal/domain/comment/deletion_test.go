package comment

import (
	"context"
	"errors"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
)

// deletionRepository 扩展现有测试仓储以记录删除命令。
type deletionRepository struct {
	*fakeRepository                 // fakeRepository 复用评论创建和列表测试能力。
	comment         *entity.Comment // comment 是待删除评论。
	deletedID       uint64          // deletedID 是传给仓储的评论标识。
	deleteErr       error           // deleteErr 是预设删除错误。
}

// FindByID 返回待删除评论。
func (f *deletionRepository) FindByID(context.Context, uint64) (*entity.Comment, error) {
	// 1. 未准备评论时返回不存在
	if f.comment == nil {
		return nil, ErrCommentNotFound
	}
	return f.comment, nil
}

// Delete 原子软删除评论并写入删除事件。
func (f *deletionRepository) Delete(_ context.Context, id uint64) error {
	// 1. 记录删除标识并返回预设结果
	f.deletedID = id
	return f.deleteErr
}

// TestDeleteAllowsOwnerAndAdminOnly 验证作者权限和管理员所有权豁免。
func TestDeleteAllowsOwnerAndAdminOnly(t *testing.T) {
	// 1. 普通作者可以删除自己的评论
	repository := &deletionRepository{fakeRepository: &fakeRepository{}, comment: &entity.Comment{ID: 9, UserID: 7, Status: StatusNormal}}
	service := NewService(repository, fakeArticleReader{published: true}, fakeUserReader{}, fakeGuard{acquired: true})
	if err := service.Delete(context.Background(), DeleteCommand{CommentID: 9, ActorID: 7}); err != nil {
		t.Fatalf("作者删除失败: %v", err)
	}
	if repository.deletedID != 9 {
		t.Fatalf("deleted id = %d, want 9", repository.deletedID)
	}

	// 2. 非作者普通用户没有删除权限
	repository.deletedID = 0
	err := service.Delete(context.Background(), DeleteCommand{CommentID: 9, ActorID: 8})
	if !errors.Is(err, ErrCommentPermissionDenied) {
		t.Fatalf("非作者错误 = %v", err)
	}
	if repository.deletedID != 0 {
		t.Fatalf("权限失败仍执行删除: %d", repository.deletedID)
	}

	// 3. 管理员仅绕过所有权校验
	if err := service.Delete(context.Background(), DeleteCommand{CommentID: 9, ActorID: 8, IsAdmin: true}); err != nil {
		t.Fatalf("管理员删除失败: %v", err)
	}
}

// TestDeleteIsIdempotentForAlreadyDeletedComment 验证重复删除仍调用幂等仓储但不改变权限规则。
func TestDeleteIsIdempotentForAlreadyDeletedComment(t *testing.T) {
	// 1. 作者重复删除已删除评论按成功处理
	repository := &deletionRepository{fakeRepository: &fakeRepository{}, comment: &entity.Comment{ID: 9, UserID: 7, Status: StatusDeleted}}
	service := NewService(repository, fakeArticleReader{published: true}, fakeUserReader{}, fakeGuard{acquired: true})
	if err := service.Delete(context.Background(), DeleteCommand{CommentID: 9, ActorID: 7}); err != nil {
		t.Fatalf("重复删除失败: %v", err)
	}
	if repository.deletedID != 9 {
		t.Fatalf("deleted id = %d, want 9", repository.deletedID)
	}
}
