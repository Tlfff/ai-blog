package article

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidInteractionProjectionSnapshot = errors.New("文章互动投影对账快照不合法")

// CommentProjectionFact 是文章上下文接收的评论计数稳定快照。
type CommentProjectionFact struct {
	CommentID uint64 // CommentID 是评论聚合标识。
	ArticleID uint64 // ArticleID 是所属文章标识。
	Version   int64  // Version 是评论聚合最后事件版本。
	Active    bool   // Active 表示评论当前是否贡献计数。
}

// LikeProjectionFact 是文章上下文接收的点赞计数稳定快照。
type LikeProjectionFact struct {
	LikeID    uint64 // LikeID 是点赞关系标识。
	ArticleID uint64 // ArticleID 是目标文章标识。
	UserID    uint64 // UserID 是点赞用户标识。
	Version   int64  // Version 是点赞关系最后事件版本。
	Active    bool   // Active 表示点赞关系当前是否贡献计数。
}

// InteractionProjectionSnapshot 聚合一次跨上下文只读对账快照。
type InteractionProjectionSnapshot struct {
	CapturedAt time.Time               // CapturedAt 是开始读取权威事实的时间。
	Comments   []CommentProjectionFact // Comments 是评论上下文发布的完整事实集合。
	Likes      []LikeProjectionFact    // Likes 是点赞上下文发布的完整事实集合。
}

// InteractionProjectionRepository 定义文章互动投影原子修复能力。
type InteractionProjectionRepository interface {
	// RepairInteractionProjections 合并完整事实快照并重算文章评论数和点赞数。
	RepairInteractionProjections(context.Context, InteractionProjectionSnapshot) error
}

// InteractionProjectionRepairer 定义对账任务可调用的文章投影修复接口。
type InteractionProjectionRepairer interface {
	// RepairInteractionProjections 校验稳定快照并修复文章互动投影。
	RepairInteractionProjections(context.Context, InteractionProjectionSnapshot) error
}

// InteractionProjectionService 校验跨上下文快照后委托文章仓储完成修复。
type InteractionProjectionService struct {
	repository InteractionProjectionRepository // repository 只写文章上下文拥有的投影。
}

// NewInteractionProjectionService 创建文章互动投影修复服务。
func NewInteractionProjectionService(repository InteractionProjectionRepository) *InteractionProjectionService {
	if repository == nil {
		panic("文章互动投影修复服务缺少仓储")
	}
	return &InteractionProjectionService{repository: repository}
}

// RepairInteractionProjections 校验完整快照并修复文章互动投影。
func (s *InteractionProjectionService) RepairInteractionProjections(ctx context.Context, snapshot InteractionProjectionSnapshot) error {
	// 1. 拒绝缺少快照边界或无法标识关系版本的数据
	if snapshot.CapturedAt.IsZero() {
		return ErrInvalidInteractionProjectionSnapshot
	}
	// 2. 评论和点赞关系在完整快照内必须唯一
	commentIDs := make(map[uint64]struct{}, len(snapshot.Comments))
	for _, fact := range snapshot.Comments {
		if fact.CommentID == 0 || fact.ArticleID == 0 || fact.Version <= 0 {
			return ErrInvalidInteractionProjectionSnapshot
		}
		if _, exists := commentIDs[fact.CommentID]; exists {
			return ErrInvalidInteractionProjectionSnapshot
		}
		commentIDs[fact.CommentID] = struct{}{}
	}
	likeIDs := make(map[uint64]struct{}, len(snapshot.Likes))
	for _, fact := range snapshot.Likes {
		if fact.LikeID == 0 || fact.ArticleID == 0 || fact.UserID == 0 || fact.Version <= 0 {
			return ErrInvalidInteractionProjectionSnapshot
		}
		if _, exists := likeIDs[fact.LikeID]; exists {
			return ErrInvalidInteractionProjectionSnapshot
		}
		likeIDs[fact.LikeID] = struct{}{}
	}
	// 3. 校验完成后只委托文章仓储写本上下文投影
	if err := s.repository.RepairInteractionProjections(ctx, snapshot); err != nil {
		return fmt.Errorf("持久化文章互动投影: %w", err)
	}
	return nil
}
