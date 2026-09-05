package user

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
)

// Repository 定义用户领域所需的数据访问能力。
type Repository interface {
	// NicknameExists 判断昵称是否已被排除用户之外的账号使用。
	NicknameExists(ctx context.Context, nickname string, excludeUserID uint64) (bool, error)
	// PhoneExists 判断手机号是否已被账号使用。
	PhoneExists(ctx context.Context, phone string) (bool, error)
	// Create 创建用户账号。
	Create(ctx context.Context, user *entity.User) error
	// FindNormalByID 查询正常状态的用户。
	FindNormalByID(ctx context.Context, userID uint64) (*entity.User, error)
	// UpdateProfile 更新用户公开资料。
	UpdateProfile(ctx context.Context, user *entity.User) error
	// UpdatePassword 更新正常用户的密码摘要。
	UpdatePassword(ctx context.Context, userID uint64, password string) error
	// UpdatePhone 更新正常用户的手机号。
	UpdatePhone(ctx context.Context, userID uint64, phone string) error
	// UpdateAvatar 更新正常用户的头像对象 Key。
	UpdateAvatar(ctx context.Context, userID uint64, objectKey string) error
	// UpdatePasswordWithCleanupTask 在同一事务中更新密码并记录会话收敛补偿任务。
	UpdatePasswordWithCleanupTask(ctx context.Context, userID uint64, password, currentToken string) error
	// ListSessionCleanupTasks 查询待执行的会话收敛补偿任务。
	ListSessionCleanupTasks(ctx context.Context, limit int) ([]*SessionCleanupTask, error)
	// CompleteSessionCleanupTask 标记会话收敛补偿任务已完成。
	CompleteSessionCleanupTask(ctx context.Context, taskID uint64) error
	// CompleteSessionCleanupTaskForSession 完成指定用户当前会话对应的补偿任务。
	CompleteSessionCleanupTaskForSession(ctx context.Context, userID uint64, currentToken string) error
}

// SessionCleanupTask 表示密码更新后待执行的会话收敛补偿任务。
type SessionCleanupTask struct {
	ID           uint64    // ID 是补偿任务唯一标识。
	UserID       uint64    // UserID 是需要收敛会话的用户标识。
	CurrentToken string    // CurrentToken 是应保留的当前设备 Token。
	CreatedTime  time.Time // CreatedTime 是任务创建时间。
}
