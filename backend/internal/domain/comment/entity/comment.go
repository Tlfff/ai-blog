// Package entity 定义评论上下文中的领域数据对象。
package entity

import "time"

// Comment 表示一条主评论或直属根评论的回复。
type Comment struct {
	ID            uint64    // ID 是评论唯一标识。
	ArticleID     uint64    // ArticleID 是评论所属的有效文章标识。
	UserID        uint64    // UserID 是评论作者标识。
	ReplyToUserID uint64    // ReplyToUserID 是被回复用户标识，0 表示未指定。
	Content       string    // Content 是评论正文。
	RootID        uint64    // RootID 是根评论标识，0 表示当前记录是主评论。
	IP            string    // IP 是评论创建时记录的来源地址。
	LikeCount     int64     // LikeCount 是点赞上下文同步的点赞数投影。
	ReplyCount    int64     // ReplyCount 是根评论直属回复数。
	Status        int8      // Status 是评论状态：0-删除，1-正常。
	CreatedTime   time.Time // CreatedTime 是评论创建时间。
	UpdatedTime   time.Time // UpdatedTime 是评论最后修改时间。
}

// PublicUser 是评论展示所需的用户公开资料快照。
type PublicUser struct {
	ID       uint64 // ID 是用户唯一标识。
	Nickname string // Nickname 是用户公开昵称。
	Avatar   string // Avatar 是用户头像地址。
}

// Item 是评论及其公开展示信息。
type Item struct {
	Comment     *Comment    // Comment 是评论领域数据。
	User        *PublicUser // User 是评论作者公开资料。
	ReplyToUser *PublicUser // ReplyToUser 是被回复用户公开资料。
}
