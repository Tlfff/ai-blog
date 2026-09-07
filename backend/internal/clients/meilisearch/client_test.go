package meilisearch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/search"
	meilisearchsdk "github.com/meilisearch/meilisearch-go"
)

// roundTripFunc 将测试函数适配为 HTTP RoundTripper。
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip 执行测试场景预设的 HTTP 往返逻辑。
func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	// 1. 将请求交给测试函数并返回预设响应
	return f(request)
}

// newTestClient 使用可控 HTTP 传输创建 Meilisearch SDK 适配器。
func newTestClient(apiKey string, transport roundTripFunc) *Client {
	// 1. 注入测试传输，避免访问真实 Meilisearch
	return newClient("http://meili.test", apiKey, &http.Client{Transport: transport})
}

// TestNewClientRejectsInvalidEndpoint 验证构造阶段拒绝缺失或非法的 Meilisearch 地址。
func TestNewClientRejectsInvalidEndpoint(t *testing.T) {
	// 1. 无效连接地址不能延迟到请求阶段才失败
	for _, endpoint := range []string{"", "meili.test", "ftp://meili.test", "://bad"} {
		if client, err := NewClient(endpoint, ""); err == nil || client != nil {
			t.Fatalf("endpoint=%q client=%#v err=%v", endpoint, client, err)
		}
	}
}

// TestClientAppendsPublishedFilterAndFormatsResults 验证固定已发布过滤、分页和格式化结果转换。
func TestClientAppendsPublishedFilterAndFormatsResults(t *testing.T) {
	// 1. 捕获 SDK 请求并验证客户端不能覆盖已发布状态过滤
	client := newTestClient("secret", func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://meili.test/indexes/articles/search" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("url=%s authorization=%q", request.URL, request.Header.Get("Authorization"))
		}
		var payload meilisearchsdk.SearchRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Query != "原" || payload.Filter != "status = 3" || payload.Offset != 10 || payload.Limit != 10 {
			t.Fatalf("payload=%#v", payload)
		}
		if len(payload.AttributesToRetrieve) != 5 || payload.AttributesToRetrieve[4] != "status" || len(payload.AttributesToHighlight) != 2 || payload.AttributesToHighlight[0] != "title" || payload.AttributesToHighlight[1] != "content_plain" || len(payload.AttributesToCrop) != 1 || payload.AttributesToCrop[0] != "content_plain:50" || payload.HighlightPreTag != "<em>" || payload.HighlightPostTag != "</em>" {
			t.Fatalf("formatting payload=%#v", payload)
		}
		body := `{"estimatedTotalHits":1,"hits":[{"id":7,"title":"原题","tags":"后端 Go","status":3,"_formatted":{"title":"<em>原</em>题","content_plain":"包含<em>现象</em>的摘要..."}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})

	// 2. 执行第二页搜索并验证高亮标题和摘要
	got, err := client.Search(context.Background(), search.Query{Keyword: "原", Page: 2, PageSize: 10})
	if err != nil || got.Total != 1 || len(got.Items) != 1 || got.Items[0].TitleHighlight != "<em>原</em>题" || got.Items[0].Summary != "包含<em>现象</em>的摘要..." || len(got.Items[0].Tags) != 2 || got.Items[0].Tags[0] != "后端" || got.Items[0].Tags[1] != "Go" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

// TestClientNeverReturnsUnpublishedHits 验证公开接口对异常响应执行二次状态隔离。
func TestClientNeverReturnsUnpublishedHits(t *testing.T) {
	// 1. 模拟 Meilisearch 错误返回草稿、软删除和已发表文档
	client := newTestClient("", func(*http.Request) (*http.Response, error) {
		body := `{"estimatedTotalHits":3,"hits":[{"id":1,"title":"删除","status":1},{"id":2,"title":"草稿","status":2},{"id":3,"title":"公开","status":3}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	result, err := client.Search(context.Background(), search.Query{Keyword: "x", Page: 1, PageSize: 10})
	if err != nil || len(result.Items) != 1 || result.Items[0].ID != 3 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

// TestClientReturnsUnavailableForTransportAndResponseFailures 验证传输、状态码和解码失败均保留错误分类。
func TestClientReturnsUnavailableForTransportAndResponseFailures(t *testing.T) {
	// 1. 逐项验证搜索基础设施失败的公开错误分类
	cases := []struct {
		name       string // name 是测试场景名称。
		statusCode int    // statusCode 是 Meilisearch 返回的 HTTP 状态码。
		body       string // body 是 Meilisearch 返回的响应体。
		err        error  // err 是传输层预设错误。
	}{
		{name: "transport", err: errors.New("connection reset")},
		{name: "http status", statusCode: http.StatusBadGateway, body: "bad gateway"},
		{name: "invalid json", statusCode: http.StatusOK, body: "{"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client := newTestClient("", func(*http.Request) (*http.Response, error) {
				if testCase.err != nil {
					return nil, testCase.err
				}
				return &http.Response{StatusCode: testCase.statusCode, Body: io.NopCloser(strings.NewReader(testCase.body)), Header: make(http.Header)}, nil
			})
			_, err := client.Search(context.Background(), search.Query{Keyword: "x", Page: 1, PageSize: 10})
			if !errors.Is(err, search.ErrUnavailable) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

// TestClientRejectsOffsetOverflow 验证超大页码不会溢出为错误的 Offset。
func TestClientRejectsOffsetOverflow(t *testing.T) {
	// 1. 超出 Meilisearch SDK Offset 范围的请求应作为参数错误拒绝
	client := newTestClient("", func(*http.Request) (*http.Response, error) {
		t.Fatal("overflow query must not reach Meilisearch")
		return nil, nil
	})
	_, err := client.Search(context.Background(), search.Query{Keyword: "x", Page: math.MaxUint64, PageSize: 20})
	if !errors.Is(err, search.ErrInvalidQuery) {
		t.Fatalf("err=%v", err)
	}
}

// TestClientFallsBackToRawFields 验证缺少格式化字段时使用原始标题和正文。
func TestClientFallsBackToRawFields(t *testing.T) {
	// 1. 返回没有 _formatted 字段的已发表命中文档
	client := newTestClient("", func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"hits":[{"id":1,"title":"标题","content_plain":"正文","tags":["go"],"status":3}],"estimatedTotalHits":1}`)), Header: make(http.Header)}, nil
	})
	result, err := client.Search(context.Background(), search.Query{Keyword: "x", Page: 1, PageSize: 10})
	if err != nil || result.Items[0].TitleHighlight != "标题" || result.Items[0].Summary != "正文" || result.Items[0].Tags == nil || len(result.Items[0].Tags) != 1 || result.Items[0].Tags[0] != "go" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
