package user

import "context"

// SessionManager 定义登录和退出对 Redis 会话的写操作。
type SessionManager interface {
	SessionRepository
	// Create 创建当前设备的登录会话。
	Create(ctx context.Context, token string, session Session, ttlSeconds int64) error
	// Delete 删除当前设备的登录会话。
	Delete(ctx context.Context, token string, userID uint64) error
	// DeleteOtherSessions 删除用户除当前 Token 外的其他登录会话。
	DeleteOtherSessions(ctx context.Context, currentToken string, userID uint64) error
}

// SessionCleanupReconciler 定义密码更新后会话收敛补偿能力。
type SessionCleanupReconciler interface {
	// ReconcileSessionCleanup 重试待处理的会话收敛补偿任务。
	ReconcileSessionCleanup(ctx context.Context) error
}
