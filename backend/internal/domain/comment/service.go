package comment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
)

const (
	defaultPageSize uint64 = 10
	minPageSize     uint64 = 10
	maxPageSize     uint64 = 20
	submissionTTL          = 2 * time.Second
)

// UseCase 定义评论上下文向应用层暴露的业务能力。
type UseCase interface {
	// Create 创建主评论或楼中楼回复。
	Create(context.Context, CreateCommand) (*entity.Comment, error)
	// ListRoots 查询文章主评论。
	ListRoots(context.Context, RootListQuery) (*ListResult, error)
	// ListReplies 查询根评论的楼中楼回复。
	ListReplies(context.Context, uint64, PageQuery) (*ListResult, error)
}

// Service 实现评论发布、回复校验和列表查询规则。
type Service struct {
	repository Repository       // repository 提供评论事务和分页查询能力。
	articles   ArticleReader    // articles 提供文章有效性查询。
	users      UserReader       // users 提供公开用户资料查询。
	guard      SubmissionGuard  // guard 提供两秒防重复提交能力。
	now        func() time.Time // now 提供可测试的当前时间。
}

// NewService 创建评论领域服务。
func NewService(repository Repository, articles ArticleReader, users UserReader, guard SubmissionGuard) *Service {
	// 1. 校验并保存评论领域服务依赖
	if repository == nil || articles == nil || users == nil || guard == nil {
		panic("评论领域服务缺少必要依赖")
	}
	return &Service{repository: repository, articles: articles, users: users, guard: guard, now: time.Now}
}

// Create 校验文章和根评论状态后创建评论。
func (s *Service) Create(ctx context.Context, command CreateCommand) (*entity.Comment, error) {
	// 1. 校验文章、根评论、回复目标和防重复规则
	if command.ArticleID == 0 || command.UserID == 0 || strings.TrimSpace(command.Content) == "" {
		return nil, ErrInvalidInput
	}
	published, err := s.articles.IsPublished(ctx, command.ArticleID)
	if err != nil {
		return nil, fmt.Errorf("查询文章状态: %w", err)
	}
	if !published {
		return nil, ErrArticleNotPublished
	}
	if command.RootID == 0 && command.ReplyToUserID > 0 {
		return nil, ErrInvalidInput
	}
	if command.RootID > 0 {
		root, err := s.repository.FindRoot(ctx, command.RootID)
		if err != nil {
			return nil, err
		}
		if root.ArticleID != command.ArticleID {
			return nil, ErrRootNotFound
		}
		if root.Status != StatusNormal {
			return nil, ErrRootDeleted
		}
		if command.ReplyToUserID > 0 {
			if command.ReplyToUserID != root.UserID {
				valid, err := s.repository.HasReplyTarget(ctx, command.RootID, command.ReplyToUserID)
				if err != nil {
					return nil, err
				}
				if !valid {
					return nil, ErrInvalidReplyTarget
				}
			}
		}
	}
	fingerprint := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%d:%d:%s", command.UserID, command.ArticleID, command.RootID, command.ReplyToUserID, command.Content)))
	acquired, err := s.guard.Acquire(ctx, "comment:create:"+hex.EncodeToString(fingerprint[:]), submissionTTL)
	if err != nil {
		return nil, fmt.Errorf("占用评论防重键: %w", err)
	}
	if !acquired {
		return nil, ErrDuplicateSubmission
	}
	created := command.CreatedTime
	if created.IsZero() {
		created = s.now()
	}
	comment := &entity.Comment{ArticleID: command.ArticleID, UserID: command.UserID, RootID: command.RootID, ReplyToUserID: command.ReplyToUserID, Content: strings.TrimSpace(command.Content), IP: command.IP, Status: StatusNormal, CreatedTime: created, UpdatedTime: created}
	if err := s.repository.Create(ctx, comment); err != nil {
		return nil, err
	}
	return comment, nil
}

// ListRoots 查询主评论并补充公开用户资料。
func (s *Service) ListRoots(ctx context.Context, query RootListQuery) (*ListResult, error) {
	// 1. 校验文章状态并查询主评论列表
	valid, err := s.articles.IsPublished(ctx, query.ArticleID)
	if err != nil {
		return nil, fmt.Errorf("查询文章状态: %w", err)
	}
	if !valid {
		return nil, ErrArticleNotPublished
	}
	query.PageQuery = normalizePage(query.PageQuery)
	result, err := s.repository.ListRoots(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.attachUsers(ctx, result)
}

// ListReplies 查询根评论回复并补充双方公开用户资料。
func (s *Service) ListReplies(ctx context.Context, rootID uint64, query PageQuery) (*ListResult, error) {
	// 1. 校验根评论和文章状态后查询直属回复
	if rootID == 0 {
		return nil, ErrRootNotFound
	}
	root, err := s.repository.FindRoot(ctx, rootID)
	if err != nil {
		return nil, err
	}
	valid, err := s.articles.IsPublished(ctx, root.ArticleID)
	if err != nil {
		return nil, fmt.Errorf("查询文章状态: %w", err)
	}
	if !valid {
		return nil, ErrArticleNotPublished
	}
	if root.Status != StatusNormal {
		return &ListResult{Items: []*entity.Item{}, Page: 1, PageSize: normalizePage(query).PageSize}, nil
	}
	result, err := s.repository.ListReplies(ctx, rootID, normalizePage(query))
	if err != nil {
		return nil, err
	}
	return s.attachUsers(ctx, result)
}

// normalizePage 执行评论上下文对应的处理。
func normalizePage(query PageQuery) PageQuery {
	// 1. 补齐并限制评论分页参数
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = defaultPageSize
	}
	if query.PageSize < minPageSize {
		query.PageSize = minPageSize
	}
	if query.PageSize > maxPageSize {
		query.PageSize = maxPageSize
	}
	return query
}

// attachUsers 执行评论上下文对应的处理。
func (s *Service) attachUsers(ctx context.Context, result *ListResult) (*ListResult, error) {
	// 1. 批量查询并附加评论双方公开用户
	ids := make([]uint64, 0, len(result.Items)*2)
	for _, item := range result.Items {
		ids = append(ids, item.Comment.UserID)
		if item.Comment.ReplyToUserID > 0 {
			ids = append(ids, item.Comment.ReplyToUserID)
		}
	}
	users, err := s.users.FindPublic(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, item := range result.Items {
		item.User = users[item.Comment.UserID]
		item.ReplyToUser = users[item.Comment.ReplyToUserID]
	}
	return result, nil
}
