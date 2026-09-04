// Package factory 转换文章领域实体与持久化对象。
package factory

import (
	"strings"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo/po"
)

// ArticleToPO 将文章实体转换为 MySQL 持久化对象。
func ArticleToPO(article *entity.Article) *po.Article {
	// 1. 标签按兼容的逗号格式保存，其余字段保持原值
	return &po.Article{
		ID: article.ID, AuthorID: article.AuthorID, Title: article.Title, Content: article.Content,
		Tags: strings.Join(article.Tags, ","), Status: article.Status, ViewCount: article.ViewCount,
		LikeCount: article.LikeCount, CommentCount: article.CommentCount,
		CreatedTime: article.CreatedTime, UpdatedTime: article.UpdatedTime,
	}
}

// ArticleFromPO 将 MySQL 持久化对象转换为文章实体。
func ArticleFromPO(article *po.Article) *entity.Article {
	// 1. 将逗号标签恢复为接口使用的标签集合
	return &entity.Article{
		ID: article.ID, AuthorID: article.AuthorID, Title: article.Title, Content: article.Content,
		Tags: splitTags(article.Tags), Status: article.Status, ViewCount: article.ViewCount,
		LikeCount: article.LikeCount, CommentCount: article.CommentCount,
		CreatedTime: article.CreatedTime, UpdatedTime: article.UpdatedTime,
	}
}

// splitTags 清理并拆分持久化标签文本。
func splitTags(value string) []string {
	// 1. 空标签保持为空集合
	if value == "" {
		return []string{}
	}
	// 2. 去除空白和空标签
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			tags = append(tags, part)
		}
	}
	return tags
}
