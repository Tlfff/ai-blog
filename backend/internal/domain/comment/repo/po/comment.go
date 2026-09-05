// Package po 定义评论上下文的数据库持久化对象。
package po

import "time"

// Comment 与 MySQL comments 表字段一一对应。
type Comment struct {
	ID            uint64    `xorm:"'id' pk autoincr"`        // ID 是评论主键。
	ArticleID     uint64    `xorm:"'article_id' notnull"`    // ArticleID 是所属文章标识。
	UserID        uint64    `xorm:"'user_id' notnull"`       // UserID 是评论作者标识。
	ReplyToUserID uint64    `xorm:"'reply_to_user_id'"`      // ReplyToUserID 是被回复用户标识。
	Content       string    `xorm:"'content' text"`          // Content 是评论正文。
	RootID        uint64    `xorm:"'root_id'"`               // RootID 是直属根评论标识，主评论为0。
	IP            string    `xorm:"'ip' varchar(50)"`        // IP 是评论创建来源地址。
	LikeCount     int64     `xorm:"'like_count'"`            // LikeCount 是点赞数投影。
	CommentCount  int64     `xorm:"'comment_count'"`         // CommentCount 是根评论回复数。
	Status        int8      `xorm:"'status' tinyint"`        // Status 是评论状态：0-删除，1-正常。
	CreatedTime   time.Time `xorm:"'created_time' datetime"` // CreatedTime 是评论创建时间。
	UpdatedTime   time.Time `xorm:"'updated_time' datetime"` // UpdatedTime 是评论最后修改时间。
}

// TableName 返回评论表名。
func (Comment) TableName() string { return "comments" }
