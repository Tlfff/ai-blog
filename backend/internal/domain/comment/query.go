package comment

import (
	"context"
	"fmt"
)

// Stats 表示评论上下文公开的互动统计。
type Stats struct {
	CommentID  uint64 // CommentID 是评论标识。
	LikeCount  int64  // LikeCount 是点赞上下文维护的点赞数投影。
	ReplyCount int64  // ReplyCount 是评论直属回复数。
}

// HotValue 返回评论当前热度：点赞数与回复数之和。
func (s Stats) HotValue() int64 {
	// 1. 两项投影均由对应上下文保证非负，开放查询只组合最终值
	return s.LikeCount + s.ReplyCount
}

// QueryUseCase 定义评论上下文向开放 gRPC 暴露的只读能力。
type QueryUseCase interface {
	// GetStats 查询正常评论的点赞数与回复数。
	GetStats(context.Context, uint64) (*Stats, error)
}

// QueryService 实现评论统计查询规则。
type QueryService struct {
	repository Repository // repository 提供评论上下文拥有的数据查询。
}

// NewQueryService 创建评论统计查询服务。
func NewQueryService(repository Repository) *QueryService {
	// 1. 启动阶段拒绝缺少评论仓储
	if repository == nil {
		panic("评论统计查询服务缺少仓储")
	}
	return &QueryService{repository: repository}
}

// GetStats 查询正常评论并返回互动统计。
func (s *QueryService) GetStats(ctx context.Context, commentID uint64) (*Stats, error) {
	// 1. 拒绝无效评论标识
	if commentID == 0 {
		return nil, ErrInvalidInput
	}

	// 2. 只返回评论上下文拥有的正常评论统计
	current, err := s.repository.FindByID(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("查询评论: %w", err)
	}
	if current == nil {
		return nil, ErrCommentNotFound
	}
	if current.Status != StatusNormal {
		return nil, ErrCommentNotFound
	}
	return &Stats{CommentID: current.ID, LikeCount: current.LikeCount, ReplyCount: current.ReplyCount}, nil
}
