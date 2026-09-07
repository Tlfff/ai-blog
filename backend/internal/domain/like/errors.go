package like

import "errors"

var (
	ErrInvalidInput       = errors.New("点赞参数不合法")
	ErrArticleUnavailable = errors.New("文章不存在或未发表")
	ErrCommentUnavailable = errors.New("评论不存在或已删除")
)
