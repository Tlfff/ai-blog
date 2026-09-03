// Package po 定义用户上下文的数据库持久化对象。
package po

import "time"

// User 与 MySQL users 表字段一一对应。
type User struct {
	ID            uint64    `xorm:"'id' pk autoincr"`                        // ID 是用户主键。
	Nickname      string    `xorm:"'nickname' varchar(50) notnull"`          // Nickname 是唯一昵称。
	Phone         string    `xorm:"'phone' varchar(50) notnull"`             // Phone 是唯一手机号。
	Password      string    `xorm:"'password' varchar(255) notnull"`         // Password 是密码摘要。
	Avatar        *string   `xorm:"'avatar' varchar(255) null"`              // Avatar 是可空头像地址。
	Role          int8      `xorm:"'role' tinyint notnull default 1"`        // Role 是用户角色。
	CreatedTime   time.Time `xorm:"'created_time' datetime notnull created"` // CreatedTime 是创建时间。
	UpdatedTime   time.Time `xorm:"'updated_time' datetime notnull updated"` // UpdatedTime 是更新时间。
	Status        int8      `xorm:"'status' tinyint notnull"`                // Status 是用户状态。
	LastLoginIP   string    `xorm:"'last_login_ip' varchar(50) notnull"`     // LastLoginIP 是最后登录 IP。
	LastLoginTime time.Time `xorm:"'last_login_time' datetime notnull"`      // LastLoginTime 是最后登录时间。
}

// TableName 返回用户表名。
func (User) TableName() string {
	return "users"
}
