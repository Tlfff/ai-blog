package repo

import (
	"context"
	"errors"

	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	user "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
)

// ArticleReaderAdapter 将文章上下文公开查询能力转换为评论查询契约。
type ArticleReaderAdapter struct {
	reader article.UseCase // reader 提供文章公开详情查询能力。
}

// NewArticleReader 创建文章有效性查询适配器。
func NewArticleReader(reader article.UseCase) *ArticleReaderAdapter {
	// 1. 校验并保存文章查询契约
	if reader == nil {
		panic("评论文章查询适配器缺少文章用例")
	}
	return &ArticleReaderAdapter{reader: reader}
}

// IsPublished 查询文章是否已发表。
func (r *ArticleReaderAdapter) IsPublished(ctx context.Context, articleID uint64) (bool, error) {
	// 1. 通过文章公开详情判断文章是否已发表
	_, err := r.reader.PublicDetail(ctx, articleID, 0)
	if err != nil {
		if errors.Is(err, article.ErrArticleNotFound) || errors.Is(err, article.ErrArticleNotPublished) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UserReaderAdapter 将用户上下文完整资料转换为公开资料。
type UserReaderAdapter struct {
	reader user.UseCase // reader 提供用户公开资料查询能力。
}

// NewUserReader 创建用户公开资料查询适配器。
func NewUserReader(reader user.UseCase) *UserReaderAdapter {
	// 1. 校验并保存用户公开资料查询契约
	if reader == nil {
		panic("评论用户查询适配器缺少用户用例")
	}
	return &UserReaderAdapter{reader: reader}
}

// FindPublic 批量查询正常用户的公开字段。
func (r *UserReaderAdapter) FindPublic(ctx context.Context, ids []uint64) (map[uint64]*entity.PublicUser, error) {
	// 1. 批量查询并转换评论用户公开资料
	result := make(map[uint64]*entity.PublicUser, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		profile, err := r.reader.GetProfile(ctx, id)
		if err != nil {
			if errors.Is(err, user.ErrUserNotFound) {
				continue
			}
			return nil, err
		}
		result[id] = &entity.PublicUser{ID: profile.ID, Nickname: profile.Nickname, Avatar: profile.Avatar}
	}
	return result, nil
}
