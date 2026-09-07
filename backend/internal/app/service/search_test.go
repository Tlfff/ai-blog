package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	searchapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/search"
	searchdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/search"
	"github.com/gin-gonic/gin"
)

// searchUseCaseFake 记录搜索领域调用并返回预设结果。
type searchUseCaseFake struct {
	result *searchdomain.Result // result 是成功响应。
	err    error                // err 是预设领域错误。
}

// Search 返回预设搜索结果或错误。
func (f *searchUseCaseFake) Search(context.Context, searchdomain.Query) (*searchdomain.Result, error) {
	// 1. 返回测试场景预设的领域结果
	return f.result, f.err
}

// TestSearchArticlesMapsHighlightAndSummary 验证 Controller 转换高亮标题和摘要。
func TestSearchArticlesMapsHighlightAndSummary(t *testing.T) {
	// 1. 注入应用层 Fake，避免跨层搭建真实领域服务
	server := &SearchService{useCase: &searchUseCaseFake{result: &searchdomain.Result{Total: 1, Page: 1, PageSize: 10, Items: []searchdomain.Item{{ID: 1, Title: "原题", TitleHighlight: "<em>原</em>题", Summary: "正文摘要", Tags: "go"}}}}}
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request = &http.Request{}
	reply, err := server.SearchArticles(ctx, &searchapi.SearchArticlesRequest{Keyword: "原", Page: 1, PageSize: 10})
	if err != nil || reply.List[0].TitleHighlight != "<em>原</em>题" || reply.List[0].Summary != "正文摘要" {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
}

// TestSearchArticlesMapsInvalidQueryError 验证无效查询错误映射为稳定业务错误。
func TestSearchArticlesMapsInvalidQueryError(t *testing.T) {
	// 1. 领域 Fake 返回参数错误
	server := &SearchService{useCase: &searchUseCaseFake{err: searchdomain.ErrInvalidQuery}}
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request = &http.Request{}
	_, err := server.SearchArticles(ctx, &searchapi.SearchArticlesRequest{Keyword: "x", Page: 1, PageSize: 10})
	if err == nil || !strings.Contains(err.Error(), "44070101") {
		t.Fatalf("err=%v", err)
	}
}

// TestSearchArticlesMapsUnavailableError 验证搜索基础设施不可用映射为稳定业务错误。
func TestSearchArticlesMapsUnavailableError(t *testing.T) {
	// 1. 领域 Fake 返回搜索服务不可用错误
	server := &SearchService{useCase: &searchUseCaseFake{err: searchdomain.ErrUnavailable}}
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request = &http.Request{}

	// 2. 验证应用层返回约定的搜索不可用业务码
	_, err := server.SearchArticles(ctx, &searchapi.SearchArticlesRequest{Keyword: "x", Page: 1, PageSize: 10})
	if err == nil || !strings.Contains(err.Error(), "44070102") {
		t.Fatalf("err=%v", err)
	}
}

// TestSearchArticlesPropagatesUnknownError 验证未知领域错误保持原始错误链。
func TestSearchArticlesPropagatesUnknownError(t *testing.T) {
	// 1. 领域 Fake 返回未知错误
	want := errors.New("backend failure")
	server := &SearchService{useCase: &searchUseCaseFake{err: want}}
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request = &http.Request{}
	_, err := server.SearchArticles(ctx, &searchapi.SearchArticlesRequest{Keyword: "x", Page: 1, PageSize: 10})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
