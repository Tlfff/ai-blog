package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
)

const (
	RoleUser          int8   = 1                                                                                                                 // RoleUser 表示普通用户角色。
	RoleAdmin         int8   = 2                                                                                                                 // RoleAdmin 表示管理员角色。
	StatusDeleted     int8   = 0                                                                                                                 // StatusDeleted 表示用户已删除。
	StatusNormal      int8   = 1                                                                                                                 // StatusNormal 表示用户状态正常。
	DefaultAvatar     string = "/placeholder-user.jpg"                                                                                           // DefaultAvatar 是新注册用户使用的默认头像。
	dummyPasswordHash        = "pbkdf2$100000$00000000000000000000000000000000$0000000000000000000000000000000000000000000000000000000000000000" // dummyPasswordHash 用于账号不存在时保持密码校验成本接近。
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
	// VerifyOldPassword 验证当前用户旧密码，并签发只能消费一次且有效 10 分钟的改密凭证。
	VerifyOldPassword(ctx context.Context, userID uint64, oldPassword string) (string, error)
	// ChangePassword 原子消费当前用户的改密凭证、更新密码并清理其他设备会话。
	ChangePassword(ctx context.Context, command ChangePasswordCommand) error
	// UpdatePhone 校验手机号唯一性后更新当前正常用户的手机号。
	UpdatePhone(ctx context.Context, command UpdatePhoneCommand) error
	// GetAvatarUploadURL 为当前用户生成受扩展名白名单约束的头像预签名上传地址。
	GetAvatarUploadURL(ctx context.Context, userID uint64, extension string) (*AvatarUploadResult, error)
	// ConfirmAvatar 校验头像对象属于当前用户后保存对象 Key，不检查对象是否存在。
	ConfirmAvatar(ctx context.Context, userID uint64, objectKey string) (string, error)
}

// Service 实现用户上下文的业务规则。
type Service struct {
	repository              Repository               // repository 提供用户数据访问能力。
	authRepository          AuthRepository           // authRepository 提供登录账号查询和登录信息更新能力。
	hasher                  PasswordHasher           // hasher 提供密码摘要能力。
	sessions                SessionManager           // sessions 提供登录会话存储能力。
	passwordTokens          PasswordChangeTokenStore // passwordTokens 提供一次性改密凭证能力。
	avatarStorage           AvatarStorage            // avatarStorage 提供头像直传能力。
	allowedAvatarExtensions AllowedImageExtensions   // allowedAvatarExtensions 是头像扩展名白名单。
	now                     func() time.Time         // now 提供可测试的当前时间。
}

// NewService 创建用户领域服务。
func NewService(repository Repository, hasher PasswordHasher) *Service {
	// 1. 启动阶段拒绝缺少必要依赖的领域服务
	if repository == nil || hasher == nil {
		panic("用户领域服务缺少必要依赖")
	}
	return &Service{repository: repository, hasher: hasher, now: time.Now}
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

// NewServiceWithSession 创建包含登录会话能力的用户领域服务。
func NewServiceWithSession(repository Repository, authRepository AuthRepository, hasher PasswordHasher, sessions SessionManager) *Service {
	// 1. 启动阶段拒绝缺少登录必要依赖的领域服务
	if authRepository == nil || sessions == nil {
		panic("用户领域服务缺少认证仓储或会话存储")
	}
	service := NewService(repository, hasher)
	service.authRepository = authRepository
	service.sessions = sessions
	return service
}

// Login 校验手机号或昵称和密码，并创建独立的 Redis 会话。
func (s *Service) Login(ctx context.Context, command LoginCommand) (*LoginResult, error) {
	// 1. 校验账号字段约束，手机号和昵称允许同时提供
	if command.Phone == "" && command.Nickname == "" ||
		(command.Phone != "" && !isDigits(command.Phone)) ||
		(command.Nickname != "" && isDigits(command.Nickname)) {
		return nil, ErrInvalidLogin
	}

	// 2. 查询正常账号并统一隐藏账号是否存在
	account, err := s.authRepository.FindNormalByAccount(ctx, command.Phone, command.Nickname)
	accountMissing := false
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			accountMissing = true
			account = &entity.User{Password: dummyPasswordHash, Status: StatusNormal}
		} else {
			return nil, err
		}
	}
	if account == nil || account.Status != StatusNormal {
		return nil, ErrInvalidCredentials
	}

	// 3. 使用恒定时间方式校验密码
	matched, err := s.hasher.Compare(account.Password, command.Password)
	if err != nil || !matched || accountMissing {
		return nil, ErrInvalidCredentials
	}

	// 4. 更新用户最后登录来源和时间
	loginAt := s.now()
	if err := s.authRepository.UpdateLogin(ctx, account.ID, command.ClientIP, loginAt); err != nil {
		return nil, fmt.Errorf("更新最后登录信息: %w", err)
	}

	// 5. 生成 32 字节安全随机 Token 并按记住登录设置有效期
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return nil, fmt.Errorf("生成登录 Token: %w", err)
	}
	token := hex.EncodeToString(rawToken)
	ttl := int64((7 * 24 * time.Hour).Seconds())
	if command.RememberMe {
		ttl = int64((30 * 24 * time.Hour).Seconds())
	}
	if err := s.sessions.Create(ctx, token, Session{UserID: account.ID, Role: account.Role, Device: command.Device, LoginIP: command.ClientIP, LoginTime: loginAt.Unix()}, ttl); err != nil {
		return nil, fmt.Errorf("保存登录会话: %w", err)
	}
	return &LoginResult{AccessToken: token, ExpiresIn: ttl}, nil
}

// Logout 只撤销当前设备携带的 Token。
func (s *Service) Logout(ctx context.Context, token string) error {
	// 1. 查询当前 Token 所属用户，避免误删其他设备会话
	if s.sessions == nil {
		return errors.New("用户退出未配置会话存储")
	}
	session, err := s.sessions.FindByToken(ctx, token)
	if err != nil {
		return err
	}
	// 2. 仅删除当前 Token 及用户 Token 集合中的对应成员
	return s.sessions.Delete(ctx, token, session.UserID)
}

// isDigits 判断字符串是否全部由 ASCII 数字组成。
func isDigits(value string) bool {
	// 1. 空字符串不视为纯数字账号
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
