package article

import "errors"

var (
	ErrImageNotFound         = errors.New("正文图片不存在")
	ErrImageAlreadyBound     = errors.New("正文图片已归属其他文章")
	ErrArticleNotFound       = errors.New("文章不存在")
	ErrArticleNotOwned       = errors.New("无权查看该文章详情")
	ErrInvalidStatus         = errors.New("文章状态不合法")
	ErrInvalidImageExtension = errors.New("正文图片扩展名不合法")
	ErrDuplicateSubmission   = errors.New("请勿重复提交")
)
