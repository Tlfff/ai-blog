// Package entity 定义点赞上下文拥有的事实数据。
package entity

import "time"

// ArticleLike 表示用户与文章之间的点赞事实关系。
type ArticleLike struct {
	ID          uint64    // ID 是点赞关系唯一标识。
	UserID      uint64    // UserID 是点赞用户标识。
	ArticleID   uint64    // ArticleID 是被点赞文章标识。
	Status      int8      // Status 是关系状态：1-已点赞；2-未点赞。
	CreatedTime time.Time // CreatedTime 是关系首次创建时间。
	UpdatedTime time.Time // UpdatedTime 是关系最后变更时间。
}
