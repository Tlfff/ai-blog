package comment

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
)

type fakeRepository struct {
	root    *entity.Comment
	created *entity.Comment
	target  bool
}

// Create 执行评论上下文对应的处理。
func (f *fakeRepository) Create(_ context.Context, c *entity.Comment) error {
	// 1. 准备并验证当前测试场景
	c.ID = 9
	f.created = c
	return nil
}

// HasReplyTarget 执行评论上下文对应的处理。
func (f *fakeRepository) HasReplyTarget(_ context.Context, _, _ uint64) (bool, error) {
	// 1. 准备并验证当前测试场景
	return f.target, nil
}

// FindRoot 执行评论上下文对应的处理。
func (f *fakeRepository) FindRoot(_ context.Context, _ uint64) (*entity.Comment, error) {
	// 1. 准备并验证当前测试场景
	if f.root == nil {
		return nil, ErrRootNotFound
	}
	return f.root, nil
}

// ListRoots 执行评论上下文对应的处理。
func (f *fakeRepository) ListRoots(context.Context, RootListQuery) (*ListResult, error) {
	// 1. 准备并验证当前测试场景
	return &ListResult{Items: []*entity.Item{{Comment: &entity.Comment{UserID: 7}}}}, nil
}

// ListReplies 执行评论上下文对应的处理。
func (f *fakeRepository) ListReplies(context.Context, uint64, PageQuery) (*ListResult, error) {
	// 1. 准备并验证当前测试场景
	return &ListResult{Items: []*entity.Item{{Comment: &entity.Comment{UserID: 7, ReplyToUserID: 8}}}}, nil
}

type fakeArticleReader struct{ published bool }

// IsPublished 执行评论上下文对应的处理。
func (f fakeArticleReader) IsPublished(context.Context, uint64) (bool, error) {
	// 1. 准备并验证当前测试场景
	return f.published, nil
}

type fakeUserReader struct{}

// FindPublic 执行评论上下文对应的处理。
func (fakeUserReader) FindPublic(context.Context, []uint64) (map[uint64]*entity.PublicUser, error) {
	// 1. 准备并验证当前测试场景
	return map[uint64]*entity.PublicUser{7: {ID: 7, Nickname: "甲"}, 8: {ID: 8, Nickname: "乙"}}, nil
}

type fakeGuard struct{ acquired bool }

// Acquire 执行评论上下文对应的处理。
func (f fakeGuard) Acquire(context.Context, string, time.Duration) (bool, error) {
	// 1. 准备并验证当前测试场景
	return f.acquired, nil
}

// newTestService 执行评论上下文对应的处理。
func newTestService(root *entity.Comment, published, acquired bool) (*Service, *fakeRepository) {
	// 1. 准备并验证当前测试场景
	repo := &fakeRepository{root: root, target: true}
	return NewService(repo, fakeArticleReader{published: published}, fakeUserReader{}, fakeGuard{acquired: acquired}), repo
}

// TestServiceCreateRejectsUnpublishedArticle 执行评论上下文对应的处理。
func TestServiceCreateRejectsUnpublishedArticle(t *testing.T) {
	// 1. 准备并验证当前测试场景
	s, _ := newTestService(nil, false, true)
	_, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, Content: "正文"})
	if !errors.Is(err, ErrArticleNotPublished) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceCreateReplyRequiresNormalRootAndKeepsRootID 执行评论上下文对应的处理。
func TestServiceCreateReplyRequiresNormalRootAndKeepsRootID(t *testing.T) {
	// 1. 准备并验证当前测试场景
	s, repo := newTestService(&entity.Comment{ID: 3, ArticleID: 1, Status: StatusNormal}, true, true)
	comment, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, RootID: 3, ReplyToUserID: 8, Content: "回复"})
	if err != nil {
		t.Fatal(err)
	}
	if comment.RootID != 3 || comment.ReplyToUserID != 8 || repo.created.RootID != 3 {
		t.Fatalf("comment = %#v", comment)
	}
}

// TestServiceCreateRejectsReplyToUnrelatedUser 执行评论上下文对应的处理。
func TestServiceCreateRejectsReplyToUnrelatedUser(t *testing.T) {
	// 1. 准备并验证当前测试场景
	repo := &fakeRepository{root: &entity.Comment{ID: 3, ArticleID: 1, UserID: 7, Status: StatusNormal}}
	repo.target = false
	s := NewService(repo, fakeArticleReader{published: true}, fakeUserReader{}, fakeGuard{acquired: true})
	_, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, RootID: 3, ReplyToUserID: 99, Content: "回复"})
	if !errors.Is(err, ErrInvalidReplyTarget) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceCreateRejectsRootReplyTarget 执行评论上下文对应的处理。
func TestServiceCreateRejectsRootReplyTarget(t *testing.T) {
	// 1. 准备并验证当前测试场景
	s, _ := newTestService(nil, true, true)
	_, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, ReplyToUserID: 8, Content: "主评论"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceListRejectsUnpublishedArticle 执行评论上下文对应的处理。
func TestServiceListRejectsUnpublishedArticle(t *testing.T) {
	// 1. 准备并验证当前测试场景
	s, _ := newTestService(nil, false, true)
	_, err := s.ListRoots(context.Background(), RootListQuery{ArticleID: 1})
	if !errors.Is(err, ErrArticleNotPublished) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceCreateRejectsDeletedRoot 执行评论上下文对应的处理。
func TestServiceCreateRejectsDeletedRoot(t *testing.T) {
	// 1. 准备并验证当前测试场景
	s, _ := newTestService(&entity.Comment{ID: 3, ArticleID: 1, Status: StatusDeleted}, true, true)
	_, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, RootID: 3, Content: "回复"})
	if !errors.Is(err, ErrRootDeleted) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceListRepliesRejectsUnpublishedArticle 执行评论上下文对应的处理。
func TestServiceListRepliesRejectsUnpublishedArticle(t *testing.T) {
	// 1. 准备并验证当前测试场景
	s, _ := newTestService(&entity.Comment{ID: 3, ArticleID: 1, Status: StatusNormal}, false, true)
	_, err := s.ListReplies(context.Background(), 3, PageQuery{})
	if !errors.Is(err, ErrArticleNotPublished) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceListRepliesLoadsReplyToUser 执行评论上下文对应的处理。
func TestServiceListRepliesLoadsReplyToUser(t *testing.T) {
	// 1. 准备并验证当前测试场景
	s, _ := newTestService(&entity.Comment{ID: 3, ArticleID: 1, Status: StatusNormal}, true, true)
	result, err := s.ListReplies(context.Background(), 3, PageQuery{})
	if err != nil || result.Items[0].ReplyToUser.Nickname != "乙" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}
