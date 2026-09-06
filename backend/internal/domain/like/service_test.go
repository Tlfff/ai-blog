package like

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/entity"
)

// fakeRepository 记录点赞状态变更并提供重建事实。
type fakeRepository struct {
	statuses  []int8                // statuses 是收到的最终点赞状态。
	facts     []*entity.ArticleLike // facts 是缓存重建使用的 MySQL 事实。
	changeErr error                 // changeErr 是点赞事务预设错误。
	listErr   error                 // listErr 是事实查询预设错误。
}

// ChangeArticleLike 记录最终状态并返回预设结果。
func (f *fakeRepository) ChangeArticleLike(_ context.Context, _, _ uint64, status int8, _ time.Time) (bool, error) {
	// 1. 保存每次领域调用的最终状态
	f.statuses = append(f.statuses, status)
	return true, f.changeErr
}

// ListActiveArticleLikes 返回缓存重建事实。
func (f *fakeRepository) ListActiveArticleLikes(context.Context) ([]*entity.ArticleLike, error) {
	// 1. 返回预设 MySQL 事实或错误
	return f.facts, f.listErr
}

// fakeArticleReader 返回文章公开状态。
type fakeArticleReader struct {
	published bool  // published 是文章是否已发表。
	err       error // err 是查询文章预设错误。
}

// IsPublished 返回预设文章状态。
func (f *fakeArticleReader) IsPublished(context.Context, uint64) (bool, error) {
	// 1. 返回预设查询结果
	return f.published, f.err
}

// fakeCache 记录点赞缓存更新和重建。
type fakeCache struct {
	states     []bool                // states 是单用户缓存最终状态。
	replaced   []*entity.ArticleLike // replaced 是完整重建事实。
	storeErr   error                 // storeErr 是单用户缓存预设错误。
	replaceErr error                 // replaceErr 是完整重建预设错误。
}

// IsArticleLiked 返回未命中。
func (*fakeCache) IsArticleLiked(context.Context, uint64, uint64) (bool, error) {
	// 1. 当前领域服务测试不使用缓存查询
	return false, nil
}

// StoreArticleLike 记录最终缓存状态。
func (f *fakeCache) StoreArticleLike(_ context.Context, _, _ uint64, liked bool) error {
	// 1. 保存点赞或取消点赞状态
	f.states = append(f.states, liked)
	return f.storeErr
}

// ReplaceArticleLikes 记录完整重建事实。
func (f *fakeCache) ReplaceArticleLikes(_ context.Context, facts []*entity.ArticleLike) error {
	// 1. 保存隔离副本供断言
	f.replaced = append([]*entity.ArticleLike(nil), facts...)
	return f.replaceErr
}

// TestServiceTreatsRepeatedLikeAndCancelAsSuccess 验证重复操作均按最终状态幂等提交。
func TestServiceTreatsRepeatedLikeAndCancelAsSuccess(t *testing.T) {
	// 1. 连续点赞和连续取消均返回成功，并向仓储传递相同最终状态
	repository := &fakeRepository{}
	cache := &fakeCache{}
	service := NewService(repository, &fakeArticleReader{published: true}, cache)
	for range 2 {
		if err := service.LikeArticle(context.Background(), 7, 9); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := service.CancelArticleLike(context.Background(), 7, 9); err != nil {
			t.Fatal(err)
		}
	}
	want := []int8{StatusLiked, StatusLiked, StatusUnliked, StatusUnliked}
	if len(repository.statuses) != len(want) {
		t.Fatalf("statuses = %v", repository.statuses)
	}
	for index := range want {
		if repository.statuses[index] != want[index] {
			t.Fatalf("statuses = %v, want %v", repository.statuses, want)
		}
	}
}

// TestServiceDoesNotRollbackFactWhenCacheFails 验证 Redis 失败不改变点赞接口成功结果。
func TestServiceDoesNotRollbackFactWhenCacheFails(t *testing.T) {
	// 1. MySQL 事务成功后 Redis 错误被记录但不返回给调用方
	repository := &fakeRepository{}
	cache := &fakeCache{storeErr: errors.New("redis unavailable")}
	service := NewService(repository, &fakeArticleReader{published: true}, cache)
	if err := service.LikeArticle(context.Background(), 7, 9); err != nil {
		t.Fatalf("LikeArticle() error = %v", err)
	}
	if len(repository.statuses) != 1 || repository.statuses[0] != StatusLiked {
		t.Fatalf("statuses = %v", repository.statuses)
	}
}

// TestServiceRejectsUnavailableArticleBeforeFactChange 验证不存在或未发表文章不会写点赞事实。
func TestServiceRejectsUnavailableArticleBeforeFactChange(t *testing.T) {
	// 1. 文章查询返回不可公开时拒绝点赞
	repository := &fakeRepository{}
	service := NewService(repository, &fakeArticleReader{}, &fakeCache{})
	if err := service.LikeArticle(context.Background(), 7, 9); !errors.Is(err, ErrArticleUnavailable) {
		t.Fatalf("LikeArticle() error = %v", err)
	}
	if len(repository.statuses) != 0 {
		t.Fatalf("unexpected fact changes = %v", repository.statuses)
	}
}

// TestServiceRebuildsCacheFromMySQLFacts 验证 Redis 点赞集合由 MySQL 当前事实重建。
func TestServiceRebuildsCacheFromMySQLFacts(t *testing.T) {
	// 1. 查询有效点赞事实并完整交给缓存替换
	facts := []*entity.ArticleLike{{ID: 1, UserID: 7, ArticleID: 9, Status: StatusLiked}}
	repository := &fakeRepository{facts: facts}
	cache := &fakeCache{}
	service := NewService(repository, &fakeArticleReader{published: true}, cache)
	if err := service.RebuildArticleLikeCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(cache.replaced) != 1 || cache.replaced[0] != facts[0] {
		t.Fatalf("replaced = %#v", cache.replaced)
	}

	// 2. Redis 替换失败作为可重试补偿错误返回
	cache.replaceErr = errors.New("redis unavailable")
	if err := service.RebuildArticleLikeCache(context.Background()); err == nil {
		t.Fatal("cache rebuild failure was ignored")
	}
}
