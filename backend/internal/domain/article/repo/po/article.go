// Package po 定义文章上下文的数据库持久化对象。
package po

import "time"

// Article 与 MySQL articles 表字段一一对应。
type Article struct {
	ID           uint64    `xorm:"'id' pk autoincr"`        // ID 是文章主键。
	AuthorID     uint64    `xorm:"'author_id' notnull"`     // AuthorID 是文章作者标识。
	Title        string    `xorm:"'title' varchar(255)"`    // Title 是文章标题。
	Content      string    `xorm:"'content' text"`          // Content 是 Markdown 正文。
	Tags         string    `xorm:"'tags' varchar(255)"`     // Tags 是逗号分隔的标签文本。
	Status       int8      `xorm:"'status' tinyint"`        // Status 是文章状态：1-删除，2-草稿，3-发表，0-兼容状态。
	ViewCount    int64     `xorm:"'view_count'"`            // ViewCount 是浏览数投影。
	LikeCount    int64     `xorm:"'like_count'"`            // LikeCount 是点赞数投影。
	CommentCount int64     `xorm:"'comment_count'"`         // CommentCount 是评论数投影。
	CreatedTime  time.Time `xorm:"'created_time' datetime"` // CreatedTime 是文章创建时间。
	UpdatedTime  time.Time `xorm:"'updated_time' datetime"` // UpdatedTime 是文章最后修改时间。
}

// TableName 返回文章表名。
func (Article) TableName() string {
	// 1. 使用功能文档约定的 articles 表
	return "articles"
}

// Image 与 MySQL article_images 表字段一一对应。
type Image struct {
	ID          uint64    `xorm:"'id' pk autoincr"`          // ID 是正文图片主键。
	ArticleID   *uint64   `xorm:"'article_id' null"`         // ArticleID 是所属文章，未绑定时为空。
	ObjectKey   string    `xorm:"'object_key' varchar(255)"` // ObjectKey 是 MinIO 稳定对象键。
	CreatedTime time.Time `xorm:"'created_time' datetime"`   // CreatedTime 是图片记录创建时间。
}

// TableName 返回正文图片表名。
func (Image) TableName() string {
	// 1. 使用功能文档约定的 article_images 表
	return "article_images"
}

// User 是文章详情读取的用户公开快照。
type User struct {
	ID          uint64 `xorm:"'id' pk"`         // ID 是作者用户标识。
	Nickname    string `xorm:"'nickname'"`      // Nickname 是作者公开昵称。
	Avatar      string `xorm:"'avatar'"`        // Avatar 是作者头像地址。
	LastLoginIP string `xorm:"'last_login_ip'"` // LastLoginIP 是当前兼容详情使用的 IP 信息。
}

// ViewHistory 与 MySQL article_view_histories 表字段一一对应。
type ViewHistory struct {
	ID          uint64    `xorm:"'id' pk autoincr"`        // ID 是浏览历史主键。
	UserID      uint64    `xorm:"'user_id' notnull"`       // UserID 是登录用户标识。
	ArticleID   uint64    `xorm:"'article_id' notnull"`    // ArticleID 是被浏览文章标识。
	CreatedTime time.Time `xorm:"'created_time' datetime"` // CreatedTime 是首次浏览时间。
	UpdatedTime time.Time `xorm:"'updated_time' datetime"` // UpdatedTime 是最近浏览时间。
}

// ViewEventInbox 与 MySQL article_view_event_inbox 技术幂等表字段一一对应。
type ViewEventInbox struct {
	EventID       string    `xorm:"'event_id' pk varchar(64)"` // EventID 是浏览事件幂等标识。
	ArticleID     uint64    `xorm:"'article_id' notnull"`      // ArticleID 是事件关联文章标识。
	ProcessedTime time.Time `xorm:"'processed_time' datetime"` // ProcessedTime 是事务处理完成时间。
}

// TableName 返回文章浏览事件技术幂等表名。
func (ViewEventInbox) TableName() string {
	// 1. 技术 Inbox 与浏览投影写入同一 MySQL 事务
	return "article_view_event_inbox"
}

// TableName 返回文章浏览历史表名。
func (ViewHistory) TableName() string {
	// 1. 使用功能文档约定的 article_view_histories 表
	return "article_view_histories"
}

// TableName 返回用户表名。
func (User) TableName() string {
	// 1. 使用用户上下文发布的稳定公开字段
	return "users"
}
