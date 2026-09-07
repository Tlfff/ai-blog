// Package entity 定义通知上下文中的领域数据对象。
package entity

import "time"

// Notification 表示接收者可读取的通知快照。
type Notification struct {
	ID             string    // ID 是 MongoDB 文档标识。
	SourceEventID  string    // SourceEventID 是点赞事件幂等标识。
	ReceiverID     uint64    // ReceiverID 是通知接收用户标识。
	Type           int8      // Type 是通知类型：1-文章点赞；2-评论点赞；3-评论文章；4-回复评论。
	IsRead         bool      // IsRead 表示接收者是否已读。
	CreatedTime    time.Time // CreatedTime 是通知创建时间。
	SenderID       uint64    // SenderID 是触发通知的用户标识。
	SenderNickname string    // SenderNickname 是创建时的发送者昵称快照。
	SenderAvatar   string    // SenderAvatar 是创建时的发送者头像快照。
	ArticleID      uint64    // ArticleID 是关联文章标识，兼容存量类型可为 0。
	Title          string    // Title 是创建时的文章标题快照。
}
