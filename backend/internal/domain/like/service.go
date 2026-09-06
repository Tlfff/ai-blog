package like

import (
	"context"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/leo/leo/log"
)

// UseCase 定义点赞上下文向应用层暴露的业务能力。
type UseCase interface {
	// LikeArticle 幂等建立用户与文章的点赞事实。
	LikeArticle(context.Context, uint64, uint64) error
	// CancelArticleLike 幂等取消用户与文章的点赞事实。
	CancelArticleLike(context.Context, uint64, uint64) error
}

// Service 实现文章点赞、取消点赞和缓存重建规则。
type Service struct {
	repository Repository       // repository 提供点赞事实和事务 Outbox 能力。
	articles   ArticleReader    // articles 提供文章公开状态查询。
	cache      Cache            // cache 提供可丢失、可重建的 Redis 点赞集合。
	now        func() time.Time // now 提供可测试的事实发生时间。
}

// NewService 创建点赞领域服务。
func NewService(repository Repository, articles ArticleReader, cache Cache) *Service {
	// 1. 启动阶段拒绝缺少点赞必要依赖
	if repository == nil || articles == nil || cache == nil {
		panic("点赞领域服务缺少必要依赖")
	}
	return &Service{repository: repository, articles: articles, cache: cache, now: time.Now}
}

// LikeArticle 幂等建立文章点赞事实。
func (s *Service) LikeArticle(ctx context.Context, userID, articleID uint64) error {
	// 1. 使用已发表文章查询契约校验操作目标
	if err := s.validateArticle(ctx, userID, articleID); err != nil {
		return err
	}

	// 2. 点赞事实与 Outbox 原子提交后再尽力刷新 Redis
	if _, err := s.repository.ChangeArticleLike(ctx, userID, articleID, StatusLiked, s.now()); err != nil {
		return err
	}
	if err := s.cache.StoreArticleLike(ctx, articleID, userID, true); err != nil {
		log.L().WithContext(ctx).Error("刷新文章点赞缓存失败", err)
	}
	return nil
}

// CancelArticleLike 幂等取消文章点赞事实。
func (s *Service) CancelArticleLike(ctx context.Context, userID, articleID uint64) error {
	// 1. 使用相同文章契约拒绝不存在或不可公开的目标
	if err := s.validateArticle(ctx, userID, articleID); err != nil {
		return err
	}

	// 2. 数据库事实提交成功后再尽力移除 Redis 成员
	if _, err := s.repository.ChangeArticleLike(ctx, userID, articleID, StatusUnliked, s.now()); err != nil {
		return err
	}
	if err := s.cache.StoreArticleLike(ctx, articleID, userID, false); err != nil {
		log.L().WithContext(ctx).Error("刷新文章取消点赞缓存失败", err)
	}
	return nil
}

// RebuildArticleLikeCache 使用 MySQL 当前点赞事实覆盖 Redis 集合。
func (s *Service) RebuildArticleLikeCache(ctx context.Context) error {
	// 1. MySQL 是权威事实源，Redis 只接收完整替换结果
	facts, err := s.repository.ListActiveArticleLikes(ctx)
	if err != nil {
		return fmt.Errorf("查询文章点赞事实: %w", err)
	}
	if err := s.cache.ReplaceArticleLikes(ctx, facts); err != nil {
		return fmt.Errorf("重建文章点赞缓存: %w", err)
	}
	return nil
}

// validateArticle 校验登录用户和已发表文章标识。
func (s *Service) validateArticle(ctx context.Context, userID, articleID uint64) error {
	// 1. 点赞关系必须同时关联有效用户和文章
	if userID == 0 || articleID == 0 {
		return ErrInvalidInput
	}
	published, err := s.articles.IsPublished(ctx, articleID)
	if err != nil {
		return fmt.Errorf("查询文章状态: %w", err)
	}
	if !published {
		return ErrArticleUnavailable
	}
	return nil
}
