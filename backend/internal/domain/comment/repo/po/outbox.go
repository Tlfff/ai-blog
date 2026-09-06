package po

import "time"

// CommentEventOutbox 表示评论集成事件的事务 Outbox 记录。
type CommentEventOutbox struct {
	EventID       string     `xorm:"'event_id' pk varchar(64)"`      // EventID 是事件幂等标识。
	AggregateID   uint64     `xorm:"'aggregate_id'"`                 // AggregateID 是评论标识。
	EventType     string     `xorm:"'event_type' varchar(64)"`       // EventType 是稳定事件类型。
	Version       int64      `xorm:"'version'"`                      // Version 是评论聚合版本。
	OccurredAt    time.Time  `xorm:"'occurred_at' datetime"`         // OccurredAt 是事实发生时间。
	Payload       string     `xorm:"'payload' text"`                 // Payload 是 JSON 事件负载。
	Status        int8       `xorm:"'status'"`                       // Status 是发布状态：0-待发布；1-已发布。
	Attempts      int        `xorm:"'attempts'"`                     // Attempts 是失败发布次数。
	NextAttemptAt time.Time  `xorm:"'next_attempt_time' datetime"`   // NextAttemptAt 是允许再次发布的时间。
	PublishedAt   *time.Time `xorm:"'published_time' datetime null"` // PublishedAt 是发布成功时间。
	LastError     string     `xorm:"'last_error' text"`              // LastError 是最近发布失败原因。
	CreatedAt     time.Time  `xorm:"'created_time' datetime"`        // CreatedAt 是 Outbox 创建时间。
	UpdatedAt     time.Time  `xorm:"'updated_time' datetime"`        // UpdatedAt 是 Outbox 更新时间。
}

// TableName 返回评论事件 Outbox 表名。
func (CommentEventOutbox) TableName() string {
	// 1. 返回评论上下文拥有的事务 Outbox 表
	return "comment_event_outbox"
}
