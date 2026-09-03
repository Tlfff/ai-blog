package user

import (
	"context"

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
}
