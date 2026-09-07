package meilisearch

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/search"
	meilisearchsdk "github.com/meilisearch/meilisearch-go"
)

const articlesIndex = "articles"

// Client 是 Meilisearch 文章索引的 Go SDK 适配器。
type Client struct {
	index meilisearchsdk.IndexManager // index 提供固定 articles 索引的搜索能力。
}

// NewClient 校验服务地址并创建 Meilisearch Go SDK 适配器。
func NewClient(endpoint, apiKey string) (*Client, error) {
	// 1. 在组合根初始化期间拒绝空地址和无效协议
	parsed, err := url.ParseRequestURI(strings.TrimSpace(endpoint))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("Meilisearch 地址无效")
	}

	// 2. 使用官方 SDK、固定超时和禁用重试的客户端访问文章索引
	httpClient := &http.Client{Timeout: 5 * time.Second}
	return newClient(parsed.String(), apiKey, httpClient), nil
}

// newClient 使用可替换 HTTP 客户端创建 Meilisearch Go SDK 适配器。
func newClient(endpoint, apiKey string, httpClient *http.Client) *Client {
	options := []meilisearchsdk.Option{meilisearchsdk.WithCustomClient(httpClient), meilisearchsdk.DisableRetries()}
	if apiKey != "" {
		options = append(options, meilisearchsdk.WithAPIKey(apiKey))
	}
	service := meilisearchsdk.New(strings.TrimRight(endpoint, "/"), options...)
	return &Client{index: service.Index(articlesIndex)}
}

// response 是 Meilisearch 文章搜索响应。
type response struct {
	Hits []struct {
		ID           uint64                `json:"id"`            // ID 是文章标识。
		Title        string                `json:"title"`         // Title 是文章原始标题。
		ContentPlain string                `json:"content_plain"` // ContentPlain 是未格式化正文。
		Tags         tagList               `json:"tags"`          // Tags 是兼容新旧索引格式的标签集合。
		Status       search.DocumentStatus `json:"status"`        // Status 是搜索文档状态。
		Formatted    struct {
			Title   string `json:"title"`         // Title 是标题高亮结果。
			Content string `json:"content_plain"` // Content 是正文裁剪结果。
		} `json:"_formatted"` // Formatted 是 Meilisearch 格式化字段。
	} `json:"hits"` // Hits 是当前页命中文章。
	EstimatedTotalHits uint64 `json:"estimatedTotalHits"` // EstimatedTotalHits 是匹配总数。
}

// Search 查询已发表文章并转换为搜索领域结果。
func (c *Client) Search(ctx context.Context, query search.Query) (*search.Result, error) {
	// 1. 计算 Meilisearch Offset，并拒绝超出 SDK 表示范围的页码
	if query.Page-1 > uint64(math.MaxInt64)/query.PageSize {
		return nil, search.ErrInvalidQuery
	}
	offset := int64((query.Page - 1) * query.PageSize)

	// 2. 通过 SDK 构造不可被客户端覆盖的公开搜索请求
	request := &meilisearchsdk.SearchRequest{
		Offset:                offset,
		Limit:                 int64(query.PageSize),
		AttributesToRetrieve:  []string{"id", "title", "content_plain", "tags", "status"},
		AttributesToHighlight: []string{"title", "content_plain"},
		AttributesToCrop:      []string{"content_plain:50"},
		CropMarker:            "...",
		HighlightPreTag:       "<em>",
		HighlightPostTag:      "</em>",
		Filter:                fmt.Sprintf("status = %d", search.DocumentStatusPublished),
	}
	payload, err := c.index.SearchRawWithContext(ctx, query.Keyword, request)
	if err != nil {
		return nil, fmt.Errorf("%w: 查询 Meilisearch: %w", search.ErrUnavailable, err)
	}

	// 3. 解码响应并回退缺少格式化字段的结果
	var decoded response
	if err := json.Unmarshal(*payload, &decoded); err != nil {
		return nil, fmt.Errorf("%w: 解码响应: %w", search.ErrUnavailable, err)
	}
	result := &search.Result{Total: decoded.EstimatedTotalHits, Page: query.Page, PageSize: query.PageSize, Items: make([]search.Item, 0, len(decoded.Hits))}
	for _, hit := range decoded.Hits {
		// 3.1 即使索引过滤设置异常，也不向公开接口返回草稿或软删除文档
		if hit.Status != search.DocumentStatusPublished {
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
		result.Items = append(result.Items, search.Item{ID: hit.ID, Title: hit.Title, TitleHighlight: title, Summary: summary, Tags: append([]string{}, hit.Tags...)})
	}
	return result, nil
}

// tagList 兼容 Meilisearch 中旧字符串与新数组两种标签格式。
type tagList []string

// UnmarshalJSON 将索引标签统一解码为字符串数组。
func (tags *tagList) UnmarshalJSON(data []byte) error {
	// 1. 优先读取新索引使用的字符串数组
	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*tags = cleanTags(values)
		return nil
	}

	// 2. 兼容旧索引中使用空格或标点连接的标签文本
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*tags = splitTags(value)
	return nil
}

// splitTags 拆分旧索引中空格或标点连接的标签文本。
func splitTags(value string) []string {
	return cleanTags(strings.FieldsFunc(value, func(char rune) bool {
		return unicode.IsSpace(char) || strings.ContainsRune(",，、;；", char)
	}))
}

// cleanTags 清理空标签并保持原始顺序去重。
func cleanTags(values []string) []string {
	tags := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, value)
	}
	return tags
}
