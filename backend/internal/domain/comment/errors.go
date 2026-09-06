package comment

import "errors"

const ArticleStatusPublished int8 = 3 // ArticleStatusPublished 表示文章已发表。

var (
	ErrArticleNotPublished     = errors.New("文章未发表")
	ErrRootNotFound            = errors.New("根评论不存在")
	ErrRootDeleted             = errors.New("根评论已删除")
	ErrInvalidReplyTarget      = errors.New("回复目标不属于根评论")
	ErrUserNotFound            = errors.New("评论用户不存在")
	ErrInvalidPage             = errors.New("评论分页参数不合法")
	ErrInvalidInput            = errors.New("评论参数不合法")
	ErrDuplicateSubmission     = errors.New("评论重复提交")
	ErrCommentNotFound         = errors.New("评论不存在")
	ErrCommentPermissionDenied = errors.New("无权删除评论")
)
