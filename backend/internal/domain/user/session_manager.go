package user

import "context"

// SessionManager 定义登录和退出对 Redis 会话的写操作。
type SessionManager interface {
	SessionRepository
	// Create 创建当前设备的登录会话。
	Create(ctx context.Context, token string, session Session, ttlSeconds int64) error
	// Delete 删除当前设备的登录会话。
	Delete(ctx context.Context, token string, userID uint64) error
}
