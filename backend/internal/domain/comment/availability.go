package comment

import "context"

// AvailabilityRepository 定义评论上下文判断正常评论所需的数据能力。
type AvailabilityRepository interface {
	// IsActive 查询评论是否存在且未删除。
	IsActive(context.Context, uint64) (bool, error)
}

// AvailabilityQuery 提供不泄漏评论实体和仓储的正常状态契约。
type AvailabilityQuery struct {
	repository AvailabilityRepository // repository 提供评论上下文权威状态查询。
}

// NewAvailabilityQuery 创建评论正常状态查询器。
func NewAvailabilityQuery(repository AvailabilityRepository) *AvailabilityQuery {
	// 1. 启动阶段拒绝缺少评论状态仓储
	if repository == nil {
		panic("评论正常状态查询器缺少仓储")
	}
	return &AvailabilityQuery{repository: repository}
}

// IsActive 查询评论是否存在且未删除。
func (q *AvailabilityQuery) IsActive(ctx context.Context, commentID uint64) (bool, error) {
	// 1. 只暴露布尔结果，避免点赞上下文依赖评论实体或仓储
	return q.repository.IsActive(ctx, commentID)
}
