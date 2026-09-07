package search

import (
	"context"
	"errors"
	"strings"
)

// DocumentStatus 表示搜索索引中的文章公开状态。
type DocumentStatus int8

const (
	// DocumentStatusPublished 表示可由公开搜索返回的已发表文章。
	DocumentStatusPublished DocumentStatus = 3
)

var (
	// ErrInvalidQuery 表示搜索关键词或分页参数不符合公开接口约束。
	ErrInvalidQuery = errors.New("搜索参数不合法")
	// ErrUnavailable 表示搜索基础设施暂时不可用。
	ErrUnavailable = errors.New("搜索服务不可用")
)

// Query 表示公开搜索请求的规范化输入。
type Query struct {
	Keyword  string // Keyword 是去除首尾空格后的搜索关键词。
	Page     uint64 // Page 是从1开始的页码。
	PageSize uint64 // PageSize 是每页数量，范围为10至20。
}

// Item 表示公开搜索结果中的文章摘要。
type Item struct {
	ID             uint64   // ID 是文章标识。
	Title          string   // Title 是文章原始标题。
	TitleHighlight string   // TitleHighlight 是含高亮标签的标题。
	Summary        string   // Summary 是正文裁剪摘要。
	Tags           []string // Tags 是规范化标签集合。
}

// Result 表示搜索结果及分页信息。
type Result struct {
	Items    []Item // Items 是当前页搜索结果。
	Total    uint64 // Total 是匹配结果总数。
	Page     uint64 // Page 是当前页码。
	PageSize uint64 // PageSize 是当前页大小。
}

// Repository 定义搜索上下文访问 Meilisearch 的能力。
type Repository interface {
	// Search 按固定公开状态过滤条件查询文章索引。
	Search(context.Context, Query) (*Result, error)
}

// UseCase 定义搜索应用层所需的领域能力。
type UseCase interface {
	// Search 校验搜索输入并返回公开文章结果。
	Search(context.Context, Query) (*Result, error)
}

// Service 实现搜索查询规则。
type Service struct {
	repository Repository // repository 提供 Meilisearch 查询能力。
}

// NewService 创建搜索领域服务。
func NewService(repository Repository) *Service {
	// 1. 启动阶段拒绝缺少搜索仓储
	if repository == nil {
		panic("搜索服务缺少仓储")
	}
	return &Service{repository: repository}
}

// Search 校验关键词和分页参数后查询公开索引。
func (s *Service) Search(ctx context.Context, query Query) (*Result, error) {
	// 1. 规范化输入并拒绝不符合公开接口约束的请求
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Keyword == "" || query.Page == 0 || query.PageSize < 10 || query.PageSize > 20 {
		return nil, ErrInvalidQuery
	}

	// 2. 通过仓储查询，保留底层错误链供应用层映射
	return s.repository.Search(ctx, query)
}
