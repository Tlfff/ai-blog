package notification

import (
	"context"
	"errors"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification/entity"
)

const TypeArticleLike int8 = 1 // TypeArticleLike 表示文章点赞通知。

// ErrInvalidInput 表示通知接收者或事件字段不合法。
var ErrInvalidInput = errors.New("通知参数不合法")

// ArticleSnapshot 是通知创建所需的文章公开快照。
type ArticleSnapshot struct {
	ID       uint64 // ID 是文章标识。
	AuthorID uint64 // AuthorID 是通知接收者标识。
	Title    string // Title 是文章标题快照。
}

// SenderSnapshot 是通知创建所需的发送者公开快照。
type SenderSnapshot struct {
	ID       uint64 // ID 是发送者标识。
	Nickname string // Nickname 是发送者昵称快照。
	Avatar   string // Avatar 是发送者头像快照。
}

// ArticleReader 定义通知上下文读取文章快照的稳定契约。
type ArticleReader interface {
	// FindArticleSnapshot 查询文章作者和标题快照。
	FindArticleSnapshot(context.Context, uint64) (*ArticleSnapshot, error)
}

// UserReader 定义通知上下文读取发送者快照的稳定契约。
type UserReader interface {
	// FindSenderSnapshot 查询发送者昵称和头像快照。
	FindSenderSnapshot(context.Context, uint64) (*SenderSnapshot, error)
}

// PageQuery 表示通知 Offset 分页输入。
type PageQuery struct {
	UserID   uint64 // UserID 是当前通知接收者标识。
	Page     uint64 // Page 是从 1 开始的页码。
	PageSize uint64 // PageSize 是规范化后的每页数量。
}

// ListResult 表示通知列表和规范化分页元数据。
type ListResult struct {
	Items    []*entity.Notification // Items 是当前页通知。
	Page     uint64                 // Page 是规范化后的页码。
	PageSize uint64                 // PageSize 是规范化后的每页数量。
}

// Repository 定义通知 MongoDB 持久化能力。
type Repository interface {
	// Create 幂等创建通知快照。
	Create(context.Context, *entity.Notification) error
	// List 按接收者分页查询通知。
	List(context.Context, PageQuery) (*ListResult, error)
	// CountUnread 统计接收者未读通知。
	CountUnread(context.Context, uint64) (int64, error)
	// MarkAllRead 将接收者全部未读通知标记为已读。
	MarkAllRead(context.Context, uint64) error
}

// UseCase 定义通知上下文对应用层公开的能力。
type UseCase interface {
	// ConsumeArticleLike 消费首次文章点赞事件。
	ConsumeArticleLike(context.Context, like.IntegrationEvent) error
	// List 查询接收者通知列表。
	List(context.Context, PageQuery) (*ListResult, error)
	// CountUnread 查询接收者未读数量。
	CountUnread(context.Context, uint64) (int64, error)
	// MarkAllRead 将接收者全部通知标记为已读。
	MarkAllRead(context.Context, uint64) error
}

// Processor 定义文章点赞通知事件处理能力。
type Processor interface {
	// ConsumeArticleLike 消费点赞事件并生成通知。
	ConsumeArticleLike(context.Context, like.IntegrationEvent) error
}

// DeadLetterPublisher 定义通知消费失败后的死信能力。
type DeadLetterPublisher interface {
	// PublishNotificationDeadLetter 发布原始事件及失败原因。
	PublishNotificationDeadLetter(context.Context, []byte, string) error
}

// Service 实现文章点赞通知生成和用户隔离查询规则。
type Service struct {
	repository Repository    // repository 提供通知文档幂等写入和接收者查询。
	articles   ArticleReader // articles 提供文章作者和标题快照。
	users      UserReader    // users 提供发送者昵称和头像快照。
}

// NewService 创建通知领域服务。
func NewService(repository Repository, articles ArticleReader, users UserReader) *Service {
	// 1. 启动阶段拒绝缺少通知必要依赖
	if repository == nil || articles == nil || users == nil {
		panic("通知领域服务缺少必要依赖")
	}
	return &Service{repository: repository, articles: articles, users: users}
}

// ConsumeArticleLike 消费首次文章点赞事件并创建快照通知。
func (s *Service) ConsumeArticleLike(ctx context.Context, event like.IntegrationEvent) error {
	// 1. 只主动处理点赞关系的首次文章点赞事件
	if event.EventType != like.ArticleLikedEventType || event.Version != 1 {
		return nil
	}
	if event.EventID == "" || event.ArticleID == 0 || event.UserID == 0 || event.OccurredAt.IsZero() {
		return ErrInvalidInput
	}
	articleSnapshot, err := s.articles.FindArticleSnapshot(ctx, event.ArticleID)
	if err != nil {
		return err
	}

	// 2. 作者自赞不创建通知
	if articleSnapshot == nil || articleSnapshot.ID != event.ArticleID || articleSnapshot.AuthorID == 0 {
		return ErrInvalidInput
	}
	if articleSnapshot.AuthorID == event.UserID {
		return nil
	}
	sender, err := s.users.FindSenderSnapshot(ctx, event.UserID)
	if err != nil {
		return err
	}
	if sender == nil || sender.ID != event.UserID {
		return ErrInvalidInput
	}
	createdAt := event.OccurredAt

	// 3. 保存类型1及创建时用户、文章快照，事件ID保证幂等
	return s.repository.Create(ctx, &entity.Notification{
		SourceEventID: event.EventID, ReceiverID: articleSnapshot.AuthorID,
		Type: TypeArticleLike, CreatedTime: createdAt, SenderID: sender.ID,
		SenderNickname: sender.Nickname, SenderAvatar: sender.Avatar,
		ArticleID: articleSnapshot.ID, Title: articleSnapshot.Title,
	})
}

// List 查询当前用户通知并规范化分页。
func (s *Service) List(ctx context.Context, query PageQuery) (*ListResult, error) {
	// 1. 接收者必须来自认证上下文
	if query.UserID == 0 {
		return nil, ErrInvalidInput
	}
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize < 10 || query.PageSize >= 200 {
		query.PageSize = 10
	}
	return s.repository.List(ctx, query)
}

// CountUnread 查询当前用户未读数量。
func (s *Service) CountUnread(ctx context.Context, userID uint64) (int64, error) {
	// 1. 只允许查询有效接收者自己的未读数量
	if userID == 0 {
		return 0, ErrInvalidInput
	}
	return s.repository.CountUnread(ctx, userID)
}

// MarkAllRead 将当前用户全部未读通知标记为已读。
func (s *Service) MarkAllRead(ctx context.Context, userID uint64) error {
	// 1. 只允许修改有效接收者自己的通知
	if userID == 0 {
		return ErrInvalidInput
	}
	return s.repository.MarkAllRead(ctx, userID)
}
