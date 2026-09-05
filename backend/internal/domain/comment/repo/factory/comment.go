// Package factory 转换评论领域实体与持久化对象。
package factory

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo/po"
)

// ToPO 将评论实体转换为持久化对象。
func ToPO(comment *entity.Comment) *po.Comment {
	// 1. 执行当前评论处理阶段
	return &po.Comment{ID: comment.ID, ArticleID: comment.ArticleID, UserID: comment.UserID, ReplyToUserID: comment.ReplyToUserID, Content: comment.Content, RootID: comment.RootID, IP: comment.IP, LikeCount: comment.LikeCount, CommentCount: comment.ReplyCount, Status: comment.Status, CreatedTime: comment.CreatedTime, UpdatedTime: comment.UpdatedTime}
}

// FromPO 将持久化对象转换为评论实体。
func FromPO(comment *po.Comment) *entity.Comment {
	// 1. 执行当前评论处理阶段
	return &entity.Comment{ID: comment.ID, ArticleID: comment.ArticleID, UserID: comment.UserID, ReplyToUserID: comment.ReplyToUserID, Content: comment.Content, RootID: comment.RootID, IP: comment.IP, LikeCount: comment.LikeCount, ReplyCount: comment.CommentCount, Status: comment.Status, CreatedTime: comment.CreatedTime, UpdatedTime: comment.UpdatedTime}
}
