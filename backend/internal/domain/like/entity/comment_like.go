package entity

import "time"

// CommentLike 表示用户对评论的点赞事实。
type CommentLike struct {
	ID          uint64    // ID 是点赞关系唯一标识。
	UserID      uint64    // UserID 是点赞用户标识。
	CommentID   uint64    // CommentID 是目标评论标识。
	Status      int8      // Status 是关系状态：1-已点赞；2-未点赞。
	CreatedTime time.Time // CreatedTime 是关系首次创建时间。
	UpdatedTime time.Time // UpdatedTime 是关系最后变更时间。
}
