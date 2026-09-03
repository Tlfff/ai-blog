// Package entity 定义用户上下文中的领域数据对象。
package entity

import "time"

// User 表示用户上下文中的账号与公开资料数据。
type User struct {
	ID            uint64    // ID 是用户唯一标识。
	Nickname      string    // Nickname 是用户公开昵称。
	Phone         string    // Phone 是用户登录手机号。
	Password      string    // Password 是 PBKDF2 格式的密码摘要。
	Avatar        string    // Avatar 是用户头像地址或对象标识。
	Role          int8      // Role 是用户角色，1 为普通用户，2 为管理员。
	CreatedTime   time.Time // CreatedTime 是账号创建时间。
	UpdatedTime   time.Time // UpdatedTime 是账号最后修改时间。
	Status        int8      // Status 是用户状态，0 为删除，1 为正常。
	LastLoginIP   string    // LastLoginIP 是最后一次登录的来源 IP。
	LastLoginTime time.Time // LastLoginTime 是最后一次登录时间。
}
