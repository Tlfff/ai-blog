package comment

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
)

// fakeRepository 记录评论领域服务的仓储调用。
type fakeRepository struct {
	root    *entity.Comment // root 是预设根评论。
	created *entity.Comment // created 是已创建评论。
	target  bool            // target 表示回复目标是否属于根评论链。
}

// Create 模拟当前评论测试所需的依赖行为。
func (f *fakeRepository) Create(_ context.Context, c *entity.Comment) error {
	// 1. 执行当前测试依赖操作并记录断言数据
	c.ID = 9
	f.created = c
	return nil
}

// HasReplyTarget 模拟当前评论测试所需的依赖行为。
func (f *fakeRepository) HasReplyTarget(_ context.Context, _, _ uint64) (bool, error) {
	// 1. 执行当前测试依赖操作并记录断言数据
	return f.target, nil
}

// FindRoot 模拟当前评论测试所需的依赖行为。
func (f *fakeRepository) FindRoot(_ context.Context, _ uint64) (*entity.Comment, error) {
	// 1. 执行当前测试依赖操作并记录断言数据
	if f.root == nil {
		return nil, ErrRootNotFound
	}
	return f.root, nil
}

// ListRoots 模拟当前评论测试所需的依赖行为。
func (f *fakeRepository) ListRoots(context.Context, RootListQuery) (*ListResult, error) {
	// 1. 执行当前测试依赖操作并记录断言数据
	return &ListResult{Items: []*entity.Item{{Comment: &entity.Comment{UserID: 7}}}}, nil
}

// ListReplies 模拟当前评论测试所需的依赖行为。
func (f *fakeRepository) ListReplies(context.Context, uint64, PageQuery) (*ListResult, error) {
	// 1. 执行当前测试依赖操作并记录断言数据
	return &ListResult{Items: []*entity.Item{{Comment: &entity.Comment{UserID: 7, ReplyToUserID: 8}}}}, nil
}

// fakeArticleReader 返回文章发表状态。
type fakeArticleReader struct {
	published bool // published 表示文章是否允许评论。
}

// IsPublished 模拟当前评论测试所需的依赖行为。
func (f fakeArticleReader) IsPublished(context.Context, uint64) (bool, error) {
	// 1. 执行当前测试依赖操作并记录断言数据
	return f.published, nil
}

// fakeUserReader 返回评论用户公开资料。
type fakeUserReader struct{}

// FindPublic 模拟当前评论测试所需的依赖行为。
func (fakeUserReader) FindPublic(context.Context, []uint64) (map[uint64]*entity.PublicUser, error) {
	// 1. 执行当前测试依赖操作并记录断言数据
	return map[uint64]*entity.PublicUser{7: {ID: 7, Nickname: "甲"}, 8: {ID: 8, Nickname: "乙"}}, nil
}

// fakeGuard 返回防重复提交占用结果。
type fakeGuard struct {
	acquired bool // acquired 表示是否成功占用防重键。
}

// Acquire 模拟当前评论测试所需的依赖行为。
func (f fakeGuard) Acquire(context.Context, string, time.Duration) (bool, error) {
	// 1. 执行当前测试依赖操作并记录断言数据
	return f.acquired, nil
}

// newTestService 模拟当前评论测试所需的依赖行为。
func newTestService(root *entity.Comment, published, acquired bool) (*Service, *fakeRepository) {
	// 1. 执行当前测试依赖操作并记录断言数据
	repo := &fakeRepository{root: root, target: true}
	return NewService(repo, fakeArticleReader{published: published}, fakeUserReader{}, fakeGuard{acquired: acquired}), repo
}

// TestServiceCreateRejectsUnpublishedArticle 模拟当前评论测试所需的依赖行为。
func TestServiceCreateRejectsUnpublishedArticle(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	s, _ := newTestService(nil, false, true)
	_, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, Content: "正文"})
	if !errors.Is(err, ErrArticleNotPublished) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceCreateReplyRequiresNormalRootAndKeepsRootID 模拟当前评论测试所需的依赖行为。
func TestServiceCreateReplyRequiresNormalRootAndKeepsRootID(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	s, repo := newTestService(&entity.Comment{ID: 3, ArticleID: 1, Status: StatusNormal}, true, true)
	comment, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, RootID: 3, ReplyToUserID: 8, Content: "回复"})
	if err != nil {
		t.Fatal(err)
	}
	if comment.RootID != 3 || comment.ReplyToUserID != 8 || repo.created.RootID != 3 {
		t.Fatalf("comment = %#v", comment)
	}
}

// TestServiceCreateRejectsReplyToUnrelatedUser 模拟当前评论测试所需的依赖行为。
func TestServiceCreateRejectsReplyToUnrelatedUser(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	repo := &fakeRepository{root: &entity.Comment{ID: 3, ArticleID: 1, UserID: 7, Status: StatusNormal}}
	repo.target = false
	s := NewService(repo, fakeArticleReader{published: true}, fakeUserReader{}, fakeGuard{acquired: true})
	_, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, RootID: 3, ReplyToUserID: 99, Content: "回复"})
	if !errors.Is(err, ErrInvalidReplyTarget) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceCreateRejectsRootReplyTarget 模拟当前评论测试所需的依赖行为。
func TestServiceCreateRejectsRootReplyTarget(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	s, _ := newTestService(nil, true, true)
	_, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, ReplyToUserID: 8, Content: "主评论"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceListRejectsUnpublishedArticle 模拟当前评论测试所需的依赖行为。
func TestServiceListRejectsUnpublishedArticle(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	s, _ := newTestService(nil, false, true)
	_, err := s.ListRoots(context.Background(), RootListQuery{ArticleID: 1})
	if !errors.Is(err, ErrArticleNotPublished) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceCreateRejectsDeletedRoot 模拟当前评论测试所需的依赖行为。
func TestServiceCreateRejectsDeletedRoot(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	s, _ := newTestService(&entity.Comment{ID: 3, ArticleID: 1, Status: StatusDeleted}, true, true)
	_, err := s.Create(context.Background(), CreateCommand{ArticleID: 1, UserID: 2, RootID: 3, Content: "回复"})
	if !errors.Is(err, ErrRootDeleted) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceListRepliesRejectsUnpublishedArticle 模拟当前评论测试所需的依赖行为。
func TestServiceListRepliesRejectsUnpublishedArticle(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	s, _ := newTestService(&entity.Comment{ID: 3, ArticleID: 1, Status: StatusNormal}, false, true)
	_, err := s.ListReplies(context.Background(), 3, PageQuery{})
	if !errors.Is(err, ErrArticleNotPublished) {
		t.Fatalf("error = %v", err)
	}
}

// TestServiceListRepliesLoadsReplyToUser 模拟当前评论测试所需的依赖行为。
func TestServiceListRepliesLoadsReplyToUser(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	s, _ := newTestService(&entity.Comment{ID: 3, ArticleID: 1, Status: StatusNormal}, true, true)
	result, err := s.ListReplies(context.Background(), 3, PageQuery{})
	if err != nil || result.Items[0].ReplyToUser.Nickname != "乙" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}
