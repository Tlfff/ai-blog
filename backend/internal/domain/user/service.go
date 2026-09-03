package user

import (
	"context"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
)

const (
	RoleUser      int8   = 1                       // RoleUser 表示普通用户角色。
	RoleAdmin     int8   = 2                       // RoleAdmin 表示管理员角色。
	StatusDeleted int8   = 0                       // StatusDeleted 表示用户已删除。
	StatusNormal  int8   = 1                       // StatusNormal 表示用户状态正常。
	DefaultAvatar string = "/placeholder-user.jpg" // DefaultAvatar 是新注册用户使用的默认头像。
)

// RegisterCommand 是注册用户所需的领域输入。
type RegisterCommand struct {
	Nickname string // Nickname 是唯一用户昵称。
	Phone    string // Phone 是唯一登录手机号。
	Password string // Password 是尚未摘要的明文密码。
}

// UpdateProfileCommand 是修改用户资料所需的领域输入。
type UpdateProfileCommand struct {
	UserID   uint64 // UserID 是待修改用户标识。
	Nickname string // Nickname 是新的公开昵称。
	Avatar   string // Avatar 是新的头像地址，允许为空。
}

// UseCase 定义用户上下文向应用层暴露的业务能力。
type UseCase interface {
	// Register 注册普通用户账号。
	Register(ctx context.Context, command RegisterCommand) error
	// GetProfile 查询正常状态的用户资料。
	GetProfile(ctx context.Context, userID uint64) (*entity.User, error)
	// UpdateProfile 修改正常状态用户的公开资料。
	UpdateProfile(ctx context.Context, command UpdateProfileCommand) error
}

// Service 实现用户上下文的业务规则。
type Service struct {
	repository Repository     // repository 提供用户数据访问能力。
	hasher     PasswordHasher // hasher 提供密码摘要能力。
}

// NewService 创建用户领域服务。
func NewService(repository Repository, hasher PasswordHasher) *Service {
	// 1. 启动阶段拒绝缺少必要依赖的领域服务
	if repository == nil || hasher == nil {
		panic("用户领域服务缺少必要依赖")
	}
	return &Service{repository: repository, hasher: hasher}
}

// Register 校验唯一性、摘要密码并创建普通用户。
func (s *Service) Register(ctx context.Context, command RegisterCommand) error {
	// 1. 按对外唯一性规则检查昵称和手机号
	nicknameExists, err := s.repository.NicknameExists(ctx, command.Nickname, 0)
	if err != nil {
		return fmt.Errorf("检查用户昵称: %w", err)
	}
	if nicknameExists {
		return ErrNicknameExists
	}
	phoneExists, err := s.repository.PhoneExists(ctx, command.Phone)
	if err != nil {
		return fmt.Errorf("检查用户手机号: %w", err)
	}
	if phoneExists {
		return ErrPhoneExists
	}

	// 2. 将明文密码转换为兼容格式的单向摘要
	passwordHash, err := s.hasher.Hash(command.Password)
	if err != nil {
		return fmt.Errorf("摘要用户密码: %w", err)
	}

	// 3. 使用固定默认角色和正常状态创建账号
	registeredAt := time.Now()
	return s.repository.Create(ctx, &entity.User{
		Nickname:      command.Nickname,
		Phone:         command.Phone,
		Password:      passwordHash,
		Avatar:        DefaultAvatar,
		Role:          RoleUser,
		Status:        StatusNormal,
		CreatedTime:   registeredAt,
		UpdatedTime:   registeredAt,
		LastLoginTime: registeredAt,
	})
}

// GetProfile 查询正常状态的用户资料。
func (s *Service) GetProfile(ctx context.Context, userID uint64) (*entity.User, error) {
	user, err := s.repository.FindNormalByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != StatusNormal {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// UpdateProfile 校验用户和昵称唯一性后修改公开资料。
func (s *Service) UpdateProfile(ctx context.Context, command UpdateProfileCommand) error {
	// 1. 只允许修改正常状态的用户
	user, err := s.GetProfile(ctx, command.UserID)
	if err != nil {
		return err
	}

	// 2. 排除当前用户后检查新昵称是否已被使用
	nicknameExists, err := s.repository.NicknameExists(ctx, command.Nickname, command.UserID)
	if err != nil {
		return fmt.Errorf("检查用户昵称: %w", err)
	}
	if nicknameExists {
		return ErrNicknameExists
	}

	// 3. 保存用户公开资料变更
	user.Nickname = command.Nickname
	user.Avatar = command.Avatar
	return s.repository.UpdateProfile(ctx, user)
}
