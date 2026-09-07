package article

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
)

// AvailableListCommand 表示开放 gRPC 的 Offset 分页输入。
type AvailableListCommand struct {
	Page     uint64 // Page 是从1开始的 Offset 页码。
	PageSize uint64 // PageSize 是每页数量，范围为1至100。
	IsDesc   bool   // IsDesc 表示是否按文章标识倒序查询。
}

// AvailableListResult 表示开放 gRPC 的已发表文章列表。
type AvailableListResult struct {
	Articles []*entity.Article // Articles 是当前页已发表文章。
	Total    uint64            // Total 是已发表文章总数。
	Page     uint64            // Page 是当前页码。
	PageSize uint64            // PageSize 是当前页大小。
}

// OpenQueryUseCase 定义文章上下文向开放 gRPC 暴露的只读能力。
type OpenQueryUseCase interface {
	// ListAvailable 使用 Offset 分页查询已发表文章。
	ListAvailable(context.Context, AvailableListCommand) (*AvailableListResult, error)
}

// OpenQueryService 实现开放文章列表的分页约束。
type OpenQueryService struct {
	repository ReadingRepository // repository 提供文章上下文拥有的已发表文章查询。
}

// NewOpenQueryService 创建开放文章查询服务。
func NewOpenQueryService(repository ReadingRepository) *OpenQueryService {
	// 1. 启动阶段拒绝缺少文章查询仓储
	if repository == nil {
		panic("开放文章查询服务缺少仓储")
	}
	return &OpenQueryService{repository: repository}
}

// ListAvailable 校验 Offset 分页并查询已发表文章。
func (s *OpenQueryService) ListAvailable(ctx context.Context, command AvailableListCommand) (*AvailableListResult, error) {
	// 1. 开放 RPC 必须显式提供页码，每页兼容1至100条
	if command.Page == 0 || command.PageSize == 0 || command.PageSize > 100 {
		return nil, ErrInvalidPagination
	}

	// 2. 复用文章上下文已发表状态查询，不向 gRPC 层暴露仓储
	articles, total, err := s.repository.ListPublished(ctx, PublicListQuery{Page: command.Page, PageSize: command.PageSize, IsDesc: command.IsDesc})
	if err != nil {
		return nil, err
	}
	available := make([]*entity.Article, 0, len(articles))
	for _, current := range articles {
		if current != nil && current.Status == StatusPublished {
			available = append(available, current)
		}
	}
	return &AvailableListResult{Articles: available, Total: total, Page: command.Page, PageSize: command.PageSize}, nil
}
