package search

import (
	"context"
	"errors"
	"testing"
)

// fakeRepository 记录查询并返回预设结果。
type fakeRepository struct {
	query Query // query 是仓储收到的规范化查询。
	err   error // err 是预设仓储错误。
}

// Search 记录领域服务传入的查询。
func (f *fakeRepository) Search(_ context.Context, query Query) (*Result, error) {
	// 1. 保存规范化查询并返回预设结果
	f.query = query
	return &Result{Page: query.Page, PageSize: query.PageSize}, f.err
}

// TestServiceTrimsAndValidatesQuery 验证关键词清理和分页边界。
func TestServiceTrimsAndValidatesQuery(t *testing.T) {
	// 1. 合法请求应清理关键词并传入仓储
	repository := &fakeRepository{}
	service := NewService(repository)
	if _, err := service.Search(context.Background(), Query{Keyword: "  go  ", Page: 1, PageSize: 10}); err != nil || repository.query.Keyword != "go" {
		t.Fatalf("query=%#v err=%v", repository.query, err)
	}

	// 2. 空关键词、非法页码和页大小必须拒绝
	for _, query := range []Query{{Keyword: " ", Page: 1, PageSize: 10}, {Keyword: "x", Page: 0, PageSize: 10}, {Keyword: "x", Page: 1, PageSize: 9}, {Keyword: "x", Page: 1, PageSize: 21}} {
		if _, err := service.Search(context.Background(), query); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("query=%#v err=%v", query, err)
		}
	}
}

// TestServicePropagatesRepositoryError 验证仓储错误不被领域层吞掉。
func TestServicePropagatesRepositoryError(t *testing.T) {
	// 1. 合法请求遇到仓储失败时保留错误链
	want := errors.New("meilisearch down")
	service := NewService(&fakeRepository{err: want})
	_, err := service.Search(context.Background(), Query{Keyword: "go", Page: 1, PageSize: 10})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
