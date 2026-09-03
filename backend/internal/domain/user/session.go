package user

import "context"

// Session 表示 Redis 中的用户登录身份。
type Session struct {
	UserID uint64 `json:"user_id"` // UserID 是登录用户标识。
	Role   int8   `json:"role"`    // Role 是用户角色，1 为普通用户，2 为管理员。
}

// SessionRepository 定义访问 Token 会话查询能力。
type SessionRepository interface {
	// FindByToken 查询仍有效的访问 Token 会话。
	FindByToken(ctx context.Context, token string) (*Session, error)
}
