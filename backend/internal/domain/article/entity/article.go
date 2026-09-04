// Package entity 定义文章上下文中的领域数据对象。
package entity

import "time"

// Article 表示文章上下文拥有的文章数据。
type Article struct {
	ID           uint64    // ID 是文章唯一标识。
	AuthorID     uint64    // AuthorID 是文章作者的用户标识。
	Title        string    // Title 是文章标题。
	Content      string    // Content 是包含稳定图片引用的 Markdown 正文。
	Tags         []string  // Tags 是文章标签集合。
	Status       int8      // Status 是文章状态：1-已删除，2-草稿，3-已发表，0-兼容状态。
	ViewCount    int64     // ViewCount 是当前可用浏览数投影。
	LikeCount    int64     // LikeCount 是当前可用点赞数投影。
	CommentCount int64     // CommentCount 是当前可用评论数投影。
	CreatedTime  time.Time // CreatedTime 是文章创建时间。
	UpdatedTime  time.Time // UpdatedTime 是文章最后修改时间。
}

// Image 表示正文图片的稳定对象信息和文章归属。
type Image struct {
	ID        uint64  // ID 是正文图片唯一标识。
	ArticleID *uint64 // ArticleID 是所属文章标识，未绑定时为空。
	ObjectKey string  // ObjectKey 是 MinIO 中持久保存的稳定对象键。
}

// Detail 表示后台编辑文章所需的文章、作者和图片信息。
type Detail struct {
	Article        *Article // Article 是文章完整领域数据。
	AuthorNickname string   // AuthorNickname 是作者展示昵称快照。
	AuthorAvatar   string   // AuthorAvatar 是作者头像地址。
	AuthorIP       string   // AuthorIP 是作者当前可用 IP 信息。
	IsLiked        bool     // IsLiked 表示当前用户是否点赞该文章。
	Images         []*Image // Images 是正文引用图片的稳定映射。
}
