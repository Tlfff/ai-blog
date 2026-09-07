package article

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
)

// TestOpenQueryServiceUsesPublishedOffsetPagination 验证开放文章查询复用已发表状态和 Offset 分页契约。
func TestOpenQueryServiceUsesPublishedOffsetPagination(t *testing.T) {
	// 1. 合法请求只向文章上下文仓储传递 Offset 参数，不启用游标
	published := &entity.Article{ID: 9, Title: "已发表", Tags: []string{"go"}, Status: StatusPublished, CreatedTime: time.Now(), UpdatedTime: time.Now()}
	articles := []*entity.Article{{ID: 8, Title: "草稿", Status: StatusDraft}, published}
	repository := &fakeReadingRepository{articles: articles}
	service := NewOpenQueryService(repository)
	result, err := service.ListAvailable(context.Background(), AvailableListCommand{Page: 3, PageSize: 100, IsDesc: true})
	if err != nil {
		t.Fatal(err)
	}
	if repository.listQuery.LastID != 0 || repository.listQuery.Page != 3 || repository.listQuery.PageSize != 100 || !repository.listQuery.IsDesc {
		t.Fatalf("query = %#v", repository.listQuery)
	}
	if result.Total != 2 || len(result.Articles) != 1 || result.Articles[0] != published {
		t.Fatalf("result = %#v", result)
	}
}

// TestOpenQueryServiceRejectsInvalidPagination 验证页码和每页数量边界。
func TestOpenQueryServiceRejectsInvalidPagination(t *testing.T) {
	// 1. page 必须大于0，page_size 必须位于1至100
	service := NewOpenQueryService(&fakeReadingRepository{})
	for _, command := range []AvailableListCommand{{Page: 0, PageSize: 10}, {Page: 1, PageSize: 0}, {Page: 1, PageSize: 101}} {
		if _, err := service.ListAvailable(context.Background(), command); !errors.Is(err, ErrInvalidPagination) {
			t.Fatalf("command=%#v error=%v", command, err)
		}
	}
}
