package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"
)

const securityUploadURLTTL = 10 * time.Minute

// ChangePasswordCommand 是修改密码所需的领域输入。
type ChangePasswordCommand struct {
	UserID       uint64 // UserID 是当前认证用户标识。
	CurrentToken string // CurrentToken 是当前设备访问 Token。
	ChangeToken  string // ChangeToken 是验证旧密码后签发的一次性凭证。
	NewPassword  string // NewPassword 是待摘要保存的新密码。
}

// UpdatePhoneCommand 是修改手机号所需的领域输入。
type UpdatePhoneCommand struct {
	UserID uint64 // UserID 是当前认证用户标识。
	Phone  string // Phone 是待更新的手机号。
}

// AvatarUploadResult 是头像直传凭证及对象 Key。
type AvatarUploadResult struct {
	UploadURL string // UploadURL 是 MinIO PUT 预签名地址。
	ObjectKey string // ObjectKey 是用户头像稳定对象 Key。
}

// PasswordChangeTokenStore 定义一次性改密凭证的 Redis 能力。
type PasswordChangeTokenStore interface {
	// Create 创建带有效期的用户改密凭证。
	CreatePasswordChangeToken(ctx context.Context, token string, userID uint64, ttl time.Duration) error
	// Consume 原子消费改密凭证并返回其所属用户。
	ConsumePasswordChangeToken(ctx context.Context, token string) (uint64, error)
}

// AvatarStorage 定义头像预签名上传和公开地址能力。
type AvatarStorage interface {
	// PresignPut 生成对象 PUT 预签名地址。
	PresignPut(ctx context.Context, objectKey string, expires time.Duration) (string, error)
	// PublicURL 根据对象 Key 生成公开访问地址。
	PublicURL(objectKey string) string
}

// AllowedImageExtensions 是头像允许使用的扩展名集合。
type AllowedImageExtensions map[string]struct{}

// NewServiceWithSecurity 创建包含 T03 账号安全能力的用户领域服务。
//
// 参数说明：
//   - repository：用户领域仓储，负责用户资料、密码、手机号和头像 Key 持久化。
//   - authRepository：登录账号仓储，负责登录账号查询和登录信息更新。
//   - hasher：PBKDF2-SHA256 密码摘要与校验器。
//   - sessions：Redis 登录会话管理器，负责当前会话和多设备会话收敛。
//   - tokens：一次性改密凭证存储，必须提供原子消费能力。
//   - storage：MinIO 头像预签名上传和公开地址适配器。
//   - allowed：头像文件扩展名白名单。
func NewServiceWithSecurity(repository Repository, authRepository AuthRepository, hasher PasswordHasher, sessions SessionManager, tokens PasswordChangeTokenStore, storage AvatarStorage, allowed AllowedImageExtensions) *Service {
	// 1. 复用 T02 的认证能力并注入 T03 的安全依赖
	service := NewServiceWithSession(repository, authRepository, hasher, sessions)
	service.passwordTokens, service.avatarStorage, service.allowedAvatarExtensions = tokens, storage, allowed
	return service
}

// VerifyOldPassword 验证旧密码并签发十分钟有效的一次性改密凭证。
func (s *Service) VerifyOldPassword(ctx context.Context, userID uint64, oldPassword string) (string, error) {
	// 1. 校验当前用户和旧密码，避免为无效凭证签发改密 Token
	if s.passwordTokens == nil {
		return "", fmt.Errorf("改密凭证存储未配置")
	}
	account, err := s.GetProfile(ctx, userID)
	if err != nil {
		return "", err
	}
	matched, err := s.hasher.Compare(account.Password, oldPassword)
	if err != nil || !matched {
		return "", ErrInvalidCredentials
	}
	// 2. 生成安全随机凭证并以十分钟 TTL 写入 Redis
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成改密凭证: %w", err)
	}
	token := hex.EncodeToString(raw)
	if err := s.passwordTokens.CreatePasswordChangeToken(ctx, token, userID, securityUploadURLTTL); err != nil {
		return "", fmt.Errorf("保存改密凭证: %w", err)
	}
	return token, nil
}

// ChangePassword 原子消费凭证、更新密码并收敛其他设备会话。
func (s *Service) ChangePassword(ctx context.Context, command ChangePasswordCommand) error {
	// 1. 先校验新密码，避免明显失败请求消耗一次性凭证
	if s.passwordTokens == nil || s.sessions == nil {
		return fmt.Errorf("账号安全依赖未配置")
	}
	if len([]rune(command.NewPassword)) < 6 {
		return ErrInvalidCredentials
	}

	// 2. 摘要新密码后原子消费凭证并确认其属于当前用户
	hash, err := s.hasher.Hash(command.NewPassword)
	if err != nil {
		return fmt.Errorf("摘要新密码: %w", err)
	}
	ownerID, err := s.passwordTokens.ConsumePasswordChangeToken(ctx, command.ChangeToken)
	if err != nil || ownerID != command.UserID {
		return ErrPasswordChangeTokenInvalid
	}

	// 3. 更新密码；凭证保持 GETDEL 一次性消费语义
	if err := s.repository.UpdatePasswordWithCleanupTask(ctx, command.UserID, hash, command.CurrentToken); err != nil {
		return err
	}

	// 4. 立即收敛其他设备会话；失败时事务已留下可重试的持久化补偿任务
	if err := s.sessions.DeleteOtherSessions(ctx, command.CurrentToken, command.UserID); err != nil {
		return err
	}
	return s.repository.CompleteSessionCleanupTaskForSession(ctx, command.UserID, command.CurrentToken)
}

// UpdatePhone 校验并更新当前用户手机号。
func (s *Service) UpdatePhone(ctx context.Context, command UpdatePhoneCommand) error {
	// 1. 校验手机号格式和当前用户状态
	if command.Phone == "" || !isDigits(command.Phone) {
		return ErrInvalidPhone
	}
	account, err := s.GetProfile(ctx, command.UserID)
	if err != nil {
		return err
	}
	if account.Phone == command.Phone {
		return nil
	}
	// 2. 先执行业务唯一性检查，数据库唯一索引继续作为并发兜底
	exists, err := s.repository.PhoneExists(ctx, command.Phone)
	if err != nil {
		return err
	}
	if exists {
		return ErrPhoneExists
	}
	// 3. 持久化手机号变更
	return s.repository.UpdatePhone(ctx, command.UserID, command.Phone)
}

// GetAvatarUploadURL 为当前用户生成头像直传凭证。
func (s *Service) GetAvatarUploadURL(ctx context.Context, userID uint64, extension string) (*AvatarUploadResult, error) {
	// 1. 校验头像扩展名白名单和当前用户状态
	if s.avatarStorage == nil {
		return nil, fmt.Errorf("头像存储未配置")
	}
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	if _, ok := s.allowedAvatarExtensions[extension]; !ok {
		return nil, ErrInvalidAvatarObjectKey
	}
	if _, err := s.GetProfile(ctx, userID); err != nil {
		return nil, err
	}
	// 2. 生成当前用户专属对象 Key 并签发十分钟 PUT 地址
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("avatar/%d/%s.%s", userID, hex.EncodeToString(raw), extension)
	url, err := s.avatarStorage.PresignPut(ctx, key, securityUploadURLTTL)
	if err != nil {
		return nil, err
	}
	return &AvatarUploadResult{UploadURL: url, ObjectKey: key}, nil
}

// ConfirmAvatar 只校验对象 Key 归属并保存头像，不检查对象是否存在。
func (s *Service) ConfirmAvatar(ctx context.Context, userID uint64, objectKey string) (string, error) {
	// 1. 严格校验对象 Key 位于当前用户目录，拒绝路径穿越和前缀混淆
	if s.avatarStorage == nil {
		return "", fmt.Errorf("头像存储未配置")
	}
	prefix := fmt.Sprintf("avatar/%d/", userID)
	clean := path.Clean(objectKey)
	if objectKey == "" || clean != objectKey || !strings.HasPrefix(objectKey, prefix) || strings.TrimPrefix(objectKey, prefix) == "" {
		return "", ErrInvalidAvatarObjectKey
	}
	// 2. 确认用户仍正常后只保存对象 Key，不检查 MinIO 对象是否存在
	if _, err := s.GetProfile(ctx, userID); err != nil {
		return "", err
	}
	if err := s.repository.UpdateAvatar(ctx, userID, objectKey); err != nil {
		return "", err
	}
	// 3. 返回公开访问地址，保持不自动删除旧头像的兼容行为
	return s.avatarStorage.PublicURL(objectKey), nil
}

// ReconcileSessionCleanup 重试所有待处理的会话收敛补偿任务。
func (s *Service) ReconcileSessionCleanup(ctx context.Context) error {
	// 1. 读取密码更新事务留下的待处理任务
	tasks, err := s.repository.ListSessionCleanupTasks(ctx, 100)
	if err != nil {
		return err
	}
	// 2. 逐项执行原子会话收敛，成功后标记任务完成
	for _, task := range tasks {
		if err := s.sessions.DeleteOtherSessions(ctx, task.CurrentToken, task.UserID); err != nil {
			return err
		}
		if err := s.repository.CompleteSessionCleanupTask(ctx, task.ID); err != nil {
			return err
		}
	}
	return nil
}
