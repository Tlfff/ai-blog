package po

import "time"

// ArticleLike 表示文章点赞事实持久化记录。
type ArticleLike struct {
	ID          uint64    `xorm:"'id' pk autoincr"`        // ID 是点赞关系唯一标识。
	UserID      uint64    `xorm:"'user_id'"`               // UserID 是点赞用户标识。
	ArticleID   uint64    `xorm:"'article_id'"`            // ArticleID 是文章标识。
	Status      int8      `xorm:"'status' tinyint"`        // Status 是关系状态：1-已点赞；2-未点赞。
	CreatedTime time.Time `xorm:"'created_time' datetime"` // CreatedTime 是关系首次创建时间。
	UpdatedTime time.Time `xorm:"'updated_time' datetime"` // UpdatedTime 是关系最后变更时间。
}

// TableName 返回文章点赞事实表名。
func (ArticleLike) TableName() string { return "article_likes" }
