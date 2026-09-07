package meilisearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/search"
)

// Client 是 Meilisearch 文章索引的 HTTP 适配器。
type Client struct {
	baseURL    string       // baseURL 是 Meilisearch 服务地址。
	apiKey     string       // apiKey 是可选的 Meilisearch 访问密钥。
	httpClient *http.Client // httpClient 是可替换的 HTTP 客户端。
}

// NewClient 创建 Meilisearch HTTP 适配器。
func NewClient(baseURL, apiKey string) *Client {
	// 1. 规范化服务地址并准备默认 HTTP 客户端
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

// request 是固定文章搜索请求。
type request struct {
	Q                     string   `json:"q"`                     // Q 是用户搜索关键词。
	Filter                string   `json:"filter"`                // Filter 是后端固定的公开状态过滤条件。
	Offset                uint64   `json:"offset"`                // Offset 是 Meilisearch 偏移量。
	Limit                 uint64   `json:"limit"`                 // Limit 是 Meilisearch 返回数量。
	AttributesToRetrieve  []string `json:"attributesToRetrieve"`  // AttributesToRetrieve 限制公开返回字段。
	AttributesToHighlight []string `json:"attributesToHighlight"` // AttributesToHighlight 是标题高亮字段。
	AttributesToCrop      []string `json:"attributesToCrop"`      // AttributesToCrop 是正文裁剪字段。
	CropMarker            string   `json:"cropMarker"`            // CropMarker 是摘要裁剪标记。
	HighlightPreTag       string   `json:"highlightPreTag"`       // HighlightPreTag 是高亮开始标签。
	HighlightPostTag      string   `json:"highlightPostTag"`      // HighlightPostTag 是高亮结束标签。
}

// response 是 Meilisearch 文章搜索响应。
type response struct {
	Hits []struct {
		ID           uint64 `json:"id"`            // ID 是文章标识。
		Title        string `json:"title"`         // Title 是文章原始标题。
		ContentPlain string `json:"content_plain"` // ContentPlain 是未格式化正文。
		Tags         string `json:"tags"`          // Tags 是规范化标签。
		Status       int8   `json:"status"`        // Status 是文章状态：3-已发表。
		Formatted    struct {
			Title   string `json:"title"`         // Title 是标题高亮结果。
			Content string `json:"content_plain"` // Content 是正文裁剪结果。
		} `json:"_formatted"` // Formatted 是 Meilisearch 格式化字段。
	} `json:"hits"` // Hits 是当前页命中文章。
	EstimatedTotalHits uint64 `json:"estimatedTotalHits"` // EstimatedTotalHits 是匹配总数。
}

// Search 查询已发表文章并转换为搜索领域结果。
func (c *Client) Search(ctx context.Context, q search.Query) (*search.Result, error) {
	// 1. 构造不可被客户端覆盖的公开搜索请求
	if c.baseURL == "" {
		return nil, fmt.Errorf("%w: 缺少 Meilisearch 地址", search.ErrUnavailable)
	}
	payload, err := json.Marshal(request{Q: q.Keyword, Filter: "status = 3", Offset: (q.Page - 1) * q.PageSize, Limit: q.PageSize, AttributesToRetrieve: []string{"id", "title", "content_plain", "tags", "status"}, AttributesToHighlight: []string{"title"}, AttributesToCrop: []string{"content_plain:50"}, CropMarker: "...", HighlightPreTag: "<em>", HighlightPostTag: "</em>"})
	if err != nil {
		return nil, fmt.Errorf("编码 Meilisearch 请求: %w", err)
	}

	// 2. 创建并发送 HTTP 请求，保留传输错误链
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/indexes/articles/search", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: 创建请求: %w", search.ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: 发送请求: %w", search.ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: HTTP %d", search.ErrUnavailable, resp.StatusCode)
	}

	// 3. 解码响应并回退缺少格式化字段的结果
	var decoded response
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("%w: 解码响应: %w", search.ErrUnavailable, err)
	}
	result := &search.Result{Total: decoded.EstimatedTotalHits, Page: q.Page, PageSize: q.PageSize, Items: make([]search.Item, 0, len(decoded.Hits))}
	for _, hit := range decoded.Hits {
		// 3.1 即使索引过滤设置异常，也不向公开接口返回草稿或软删除文档
		if hit.Status != 3 {
			continue
		}
		title := hit.Formatted.Title
		if title == "" {
			title = hit.Title
		}
		summary := hit.Formatted.Content
		if summary == "" {
			summary = hit.ContentPlain
		}
		result.Items = append(result.Items, search.Item{ID: hit.ID, Title: hit.Title, TitleHighlight: title, Summary: summary, Tags: hit.Tags})
	}
	return result, nil
}
