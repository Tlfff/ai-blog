package repo

import (
	"context"
	"errors"

	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	user "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
)

// ArticleReaderAdapter 将文章上下文公开查询能力转换为通知快照契约。
type ArticleReaderAdapter struct {
	reader *article.NotificationQuery // reader 提供文章作者和标题快照。
}

// NewArticleReader 创建通知文章快照查询适配器。
func NewArticleReader(reader *article.NotificationQuery) *ArticleReaderAdapter {
	// 1. 启动阶段拒绝缺少文章查询器
	if reader == nil {
		panic("通知文章查询适配器缺少文章查询器")
	}
	return &ArticleReaderAdapter{reader: reader}
}

// FindArticleSnapshot 查询文章作者和标题快照。
func (r *ArticleReaderAdapter) FindArticleSnapshot(ctx context.Context, id uint64) (*notification.ArticleSnapshot, error) {
	// 1. 将文章上下文快照转换为通知上下文契约
	articleSnapshot, err := r.reader.FindNotificationSnapshot(ctx, id)
	if err != nil {
		return nil, err
	}
	return &notification.ArticleSnapshot{ID: articleSnapshot.ID, AuthorID: articleSnapshot.AuthorID, Title: articleSnapshot.Title}, nil
}

// UserReaderAdapter 将用户公开应用接口转换为发送者快照契约。
type UserReaderAdapter struct {
	reader user.QueryUseCase // reader 提供正常用户资料查询。
}

// NewUserReader 创建通知发送者快照查询适配器。
func NewUserReader(reader user.QueryUseCase) *UserReaderAdapter {
	// 1. 启动阶段拒绝缺少用户公开查询接口
	if reader == nil {
		panic("通知用户查询适配器缺少用户用例")
	}
	return &UserReaderAdapter{reader: reader}
}

// FindSenderSnapshot 查询正常用户的昵称和头像快照。
func (r *UserReaderAdapter) FindSenderSnapshot(ctx context.Context, id uint64) (*notification.SenderSnapshot, error) {
	// 1. 用户不存在时返回空快照，由通知领域决定是否重试
	profile, err := r.reader.GetProfile(ctx, id)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &notification.SenderSnapshot{ID: profile.ID, Nickname: profile.Nickname, Avatar: profile.Avatar}, nil
}
