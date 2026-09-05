package po

import "time"

// SessionCleanupTask 与用户密码更新后的会话收敛补偿表字段对应。
type SessionCleanupTask struct {
	ID           uint64    `xorm:"'id' pk autoincr"`                        // ID 是补偿任务主键。
	UserID       uint64    `xorm:"'user_id' notnull index"`                 // UserID 是用户标识。
	CurrentToken string    `xorm:"'current_token' varchar(128) notnull"`    // CurrentToken 是应保留的当前设备 Token。
	Status       int8      `xorm:"'status' tinyint notnull index"`          // Status 是任务状态：1-待处理；2-已完成。
	CreatedTime  time.Time `xorm:"'created_time' datetime notnull created"` // CreatedTime 是任务创建时间。
	UpdatedTime  time.Time `xorm:"'updated_time' datetime notnull updated"` // UpdatedTime 是任务更新时间。
}

// TableName 返回会话收敛补偿表名。
func (SessionCleanupTask) TableName() string {
	// 1. 使用账号安全补偿任务约定的技术表
	return "user_session_cleanup_tasks"
}
