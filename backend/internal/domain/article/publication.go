package article

import "context"

// PublicationQuery 提供不依赖点赞查询的文章公开状态契约。
type PublicationQuery struct {
	repository Repository // repository 提供文章上下文权威状态查询。
}

// NewPublicationQuery 创建文章公开状态查询器。
func NewPublicationQuery(repository Repository) *PublicationQuery {
	// 1. 启动阶段拒绝缺少文章仓储
	if repository == nil {
		panic("文章公开状态查询器缺少仓储")
	}
	return &PublicationQuery{repository: repository}
}

// IsPublished 查询文章是否存在且处于已发表状态。
func (q *PublicationQuery) IsPublished(ctx context.Context, articleID uint64) (bool, error) {
	// 1. 只暴露布尔结果，避免点赞上下文依赖文章实体或仓储
	return q.repository.IsPublished(ctx, articleID)
}
