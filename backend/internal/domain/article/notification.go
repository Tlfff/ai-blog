package article

import (
	"context"
	"fmt"
)

// NotificationSnapshot 是点赞通知所需的文章所有权与标题快照。
type NotificationSnapshot struct {
	ID       uint64 // ID 是文章标识。
	AuthorID uint64 // AuthorID 是文章作者标识。
	Title    string // Title 是文章标题。
}

// NotificationRepository 定义文章上下文提供通知快照的数据能力。
type NotificationRepository interface {
	// FindNotificationSnapshot 查询文章作者和标题。
	FindNotificationSnapshot(context.Context, uint64) (*NotificationSnapshot, error)
}

// NotificationQuery 提供通知上下文可调用的文章快照接口。
type NotificationQuery struct {
	repository NotificationRepository // repository 提供文章上下文权威快照查询。
}

// NewNotificationQuery 创建文章通知快照查询器。
func NewNotificationQuery(repository NotificationRepository) *NotificationQuery {
	// 1. 启动阶段拒绝缺少文章仓储
	if repository == nil {
		panic("文章通知快照查询器缺少仓储")
	}
	return &NotificationQuery{repository: repository}
}

// FindNotificationSnapshot 查询文章作者和标题，不泄漏文章仓储。
func (q *NotificationQuery) FindNotificationSnapshot(ctx context.Context, id uint64) (*NotificationSnapshot, error) {
	// 1. 只向通知上下文暴露稳定快照字段
	snapshot, err := q.repository.FindNotificationSnapshot(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查询文章通知快照: %w", err)
	}
	return snapshot, nil
}
