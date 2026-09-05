package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
)

// TestServicePasswordChangeConsumesCredentialAndConvergesSessions 验证成功改密只消费一次凭证并收敛会话。
func TestServicePasswordChangeConsumesCredentialAndConvergesSessions(t *testing.T) {
	// 1. 使用正确旧密码获取改密凭证并完成一次密码修改
	repo := &t03Repository{user: &entity.User{ID: 7, Password: "old", Phone: "13800138000", Status: StatusNormal}}
	hasher := &t03Hasher{}
	tokens := &t03PasswordTokens{userID: 7}
	sessions := &t03Sessions{}
	svc := NewServiceWithSecurity(repo, repo, hasher, sessions, tokens, nil, nil)

	credential, err := svc.VerifyOldPassword(context.Background(), 7, "old")
	if err != nil || credential == "" || tokens.ttl != 10*time.Minute {
		t.Fatalf("VerifyOldPassword() = %q, %v, ttl=%v", credential, err, tokens.ttl)
	}
	if err := svc.ChangePassword(context.Background(), ChangePasswordCommand{UserID: 7, CurrentToken: "current", ChangeToken: credential, NewPassword: "new-password"}); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if repo.password != hasher.hash || sessions.deletedToken != "current" || sessions.deletedUserID != 7 {
		t.Fatalf("password/session state = %q/%q/%d", repo.password, sessions.deletedToken, sessions.deletedUserID)
	}
	if err := svc.ChangePassword(context.Background(), ChangePasswordCommand{UserID: 7, CurrentToken: "current", ChangeToken: credential, NewPassword: "new-password"}); !errors.Is(err, ErrPasswordChangeTokenInvalid) {
		t.Fatalf("reused credential error = %v", err)
	}
}

// TestServicePasswordChangeRestoresCredentialAfterDatabaseFailure 验证密码事务失败后凭证按剩余有效期恢复。
func TestServicePasswordChangeRestoresCredentialAfterDatabaseFailure(t *testing.T) {
	// 1. 构造 MySQL 密码更新失败但改密凭证已被原子消费的场景
	repository := &t03Repository{
		user:        &entity.User{ID: 7, Password: "old", Status: StatusNormal},
		passwordErr: errors.New("database unavailable"),
	}
	tokens := &t03PasswordTokens{userID: 7, remaining: 9 * time.Minute}
	service := NewServiceWithSecurity(repository, repository, &t03Hasher{}, &t03Sessions{}, tokens, nil, nil)
	credential, err := service.VerifyOldPassword(context.Background(), 7, "old")
	if err != nil {
		t.Fatal(err)
	}

	// 2. 数据库失败时恢复凭证且不报告成功
	if err := service.ChangePassword(context.Background(), ChangePasswordCommand{UserID: 7, CurrentToken: "current", ChangeToken: credential, NewPassword: "new-password"}); err == nil || !tokens.restored || tokens.remaining != 9*time.Minute {
		t.Fatalf("ChangePassword() error = %v, restored = %v, ttl = %s", err, tokens.restored, tokens.remaining)
	}

	// 3. 数据库恢复后同一凭证可以重试并在成功后保持消费状态
	repository.passwordErr = nil
	if err := service.ChangePassword(context.Background(), ChangePasswordCommand{UserID: 7, CurrentToken: "current", ChangeToken: credential, NewPassword: "new-password"}); err != nil {
		t.Fatal(err)
	}
	if tokens.restored {
		t.Fatal("credential should remain consumed after successful retry")
	}
}

// TestServiceChangePhoneEnforcesBusinessAndRepositoryUniqueness 验证手机号格式和仓储唯一性校验。
func TestServiceChangePhoneEnforcesBusinessAndRepositoryUniqueness(t *testing.T) {
	// 1. 正常手机号可以更新，重复手机号返回稳定领域错误
	repo := &t03Repository{user: &entity.User{ID: 7, Status: StatusNormal}}
	svc := NewServiceWithSecurity(repo, repo, &t03Hasher{}, &t03Sessions{}, &t03PasswordTokens{}, nil, nil)
	if err := svc.UpdatePhone(context.Background(), UpdatePhoneCommand{UserID: 7, Phone: "13800138000"}); err != nil {
		t.Fatalf("UpdatePhone() error = %v", err)
	}
	if repo.phone != "13800138000" {
		t.Fatalf("phone = %q", repo.phone)
	}
	repo.phoneExists = true
	if err := svc.UpdatePhone(context.Background(), UpdatePhoneCommand{UserID: 7, Phone: "13900139000"}); !errors.Is(err, ErrPhoneExists) {
		t.Fatalf("duplicate phone error = %v", err)
	}
}

// TestServiceAvatarUploadRequiresOwnedKeyWithoutObjectExistenceCheck 验证头像路径归属和兼容的存在性语义。
func TestServiceAvatarUploadRequiresOwnedKeyWithoutObjectExistenceCheck(t *testing.T) {
	// 1. 生成当前用户路径下的头像凭证并确认对象 Key
	repo := &t03Repository{user: &entity.User{ID: 7, Status: StatusNormal, Avatar: "old"}}
	storage := &t03Storage{}
	svc := NewServiceWithSecurity(repo, repo, &t03Hasher{}, &t03Sessions{}, &t03PasswordTokens{}, storage, AllowedImageExtensions{"png": {}})
	result, err := svc.GetAvatarUploadURL(context.Background(), 7, "PNG")
	if err != nil || result.ObjectKey == "" || storage.presignKey != result.ObjectKey || storage.expires != 10*time.Minute {
		t.Fatalf("GetAvatarUploadURL() = %#v, %v", result, err)
	}
	confirmed, err := svc.ConfirmAvatar(context.Background(), 7, result.ObjectKey)
	if err != nil || confirmed != storage.publicURL || repo.avatar != result.ObjectKey {
		t.Fatalf("ConfirmAvatar() = %q, %v; avatar=%q", confirmed, err, repo.avatar)
	}
	if _, err := svc.ConfirmAvatar(context.Background(), 8, result.ObjectKey); !errors.Is(err, ErrInvalidAvatarObjectKey) {
		t.Fatalf("foreign avatar key error = %v", err)
	}
}

// t03Repository 记录账号安全领域测试的用户持久化操作。
type t03Repository struct {
	user        *entity.User // user 是可查询的正常用户。
	password    string       // password 是已保存的密码摘要。
	phone       string       // phone 是已保存的手机号。
	avatar      string       // avatar 是已保存的头像对象 Key。
	phoneExists bool         // phoneExists 是手机号唯一性预设结果。
	passwordErr error        // passwordErr 是密码事务预设错误。
}

// NicknameExists 模拟昵称唯一性查询。
func (r *t03Repository) NicknameExists(context.Context, string, uint64) (bool, error) {
	// 1. T03 测试默认不存在昵称冲突
	return false, nil
}

// PhoneExists 返回预设手机号唯一性结果。
func (r *t03Repository) PhoneExists(context.Context, string) (bool, error) {
	// 1. 返回测试预设的手机号占用状态
	return r.phoneExists, nil
}

// Create 模拟用户创建。
func (r *t03Repository) Create(context.Context, *entity.User) error {
	// 1. T03 测试不涉及用户注册持久化
	return nil
}

// FindNormalByID 返回测试用户资料。
func (r *t03Repository) FindNormalByID(context.Context, uint64) (*entity.User, error) {
	// 1. 返回正常用户副本，避免测试修改共享夹具
	if r.user == nil {
		return nil, ErrUserNotFound
	}
	u := *r.user
	return &u, nil
}

// UpdateProfile 模拟资料更新。
func (r *t03Repository) UpdateProfile(context.Context, *entity.User) error {
	// 1. T03 测试不涉及通用资料更新
	return nil
}

// UpdatePassword 模拟密码摘要更新。
func (r *t03Repository) UpdatePassword(context.Context, uint64, string) error {
	// 1. 记录测试密码摘要更新结果
	r.password = "hashed"
	return nil
}

// UpdatePhone 模拟手机号更新。
func (r *t03Repository) UpdatePhone(context.Context, uint64, string) error {
	// 1. 记录测试手机号更新结果
	r.phone = "13800138000"
	return nil
}

// UpdatePasswordWithCleanupTask 模拟密码与会话补偿任务事务写入。
func (r *t03Repository) UpdatePasswordWithCleanupTask(_ context.Context, _ uint64, password, _ string) error {
	// 1. 记录数据库事务成功写入的密码摘要
	if r.passwordErr != nil {
		return r.passwordErr
	}
	r.password = password
	return nil
}

// ListSessionCleanupTasks 返回空的会话补偿任务。
func (r *t03Repository) ListSessionCleanupTasks(context.Context, int) ([]*SessionCleanupTask, error) {
	// 1. T03 测试默认没有待处理补偿任务
	return nil, nil
}

// CompleteSessionCleanupTask 模拟完成会话补偿任务。
func (r *t03Repository) CompleteSessionCleanupTask(context.Context, uint64) error {
	// 1. T03 测试默认成功完成补偿任务
	return nil
}

// CompleteSessionCleanupTaskForSession 模拟完成当前会话补偿任务。
func (r *t03Repository) CompleteSessionCleanupTaskForSession(context.Context, uint64, string) error {
	// 1. T03 测试默认成功完成当前会话补偿任务
	return nil
}

// UpdateAvatar 模拟头像对象 Key 更新。
func (r *t03Repository) UpdateAvatar(_ context.Context, _ uint64, key string) error {
	// 1. 记录测试头像对象 Key
	r.avatar = key
	return nil
}

// FindNormalByAccount 返回测试用户账号。
func (r *t03Repository) FindNormalByAccount(context.Context, string, string) (*entity.User, error) {
	// 1. 返回测试账号供既有登录接口兼容
	return r.user, nil
}

// UpdateLogin 模拟登录信息更新。
func (r *t03Repository) UpdateLogin(context.Context, uint64, string, time.Time) error {
	// 1. T03 测试不涉及登录信息持久化
	return nil
}

// t03Hasher 模拟 PBKDF2 密码摘要和校验。
type t03Hasher struct {
	hash string // hash 是最近生成的测试摘要。
}

// Hash 返回固定测试密码摘要。
func (h *t03Hasher) Hash(string) (string, error) {
	// 1. 返回固定摘要并记录结果
	h.hash = "hashed"
	return h.hash, nil
}

// Compare 比较测试密码摘要和明文输入。
func (*t03Hasher) Compare(encoded, password string) (bool, error) {
	// 1. 以字符串相等模拟密码校验
	return encoded == password, nil
}

// t03PasswordTokens 模拟一次性改密凭证的消费和失败恢复。
type t03PasswordTokens struct {
	userID    uint64        // userID 是凭证所属用户标识。
	ttl       time.Duration // ttl 是签发时设置的有效期。
	remaining time.Duration // remaining 是原子消费返回的剩余有效期。
	consumed  bool          // consumed 表示凭证已被消费。
	restored  bool          // restored 表示失败后凭证已恢复。
}

// CreatePasswordChangeToken 记录改密凭证有效期。
func (t *t03PasswordTokens) CreatePasswordChangeToken(context.Context, string, uint64, time.Duration) error {
	// 1. 保存规格要求的十分钟有效期
	t.ttl = 10 * time.Minute
	return nil
}

// ConsumePasswordChangeToken 原子模拟消费改密凭证。
func (t *t03PasswordTokens) ConsumePasswordChangeToken(context.Context, string) (uint64, time.Duration, error) {
	// 1. 已消费且未恢复的凭证不可再次使用
	if t.consumed {
		if !t.restored {
			return 0, 0, ErrPasswordChangeTokenInvalid
		}
	}
	// 2. 首次使用凭证进入消费状态
	t.consumed = true
	t.restored = false
	if t.remaining == 0 {
		t.remaining = 9 * time.Minute
	}
	return t.userID, t.remaining, nil
}

// RestorePasswordChangeToken 按剩余有效期恢复失败事务的凭证。
func (t *t03PasswordTokens) RestorePasswordChangeToken(_ context.Context, _ string, _ uint64, ttl time.Duration) error {
	// 1. 记录恢复状态和原剩余有效期
	t.restored = true
	t.remaining = ttl
	return nil
}

// t03Sessions 记录改密后的其他设备会话收敛。
type t03Sessions struct {
	deletedToken  string // deletedToken 是收敛时保留的当前 Token。
	deletedUserID uint64 // deletedUserID 是执行会话收敛的用户标识。
}

// FindByToken 模拟访问 Token 不存在。
func (*t03Sessions) FindByToken(context.Context, string) (*Session, error) {
	// 1. T03 测试不涉及 Redis 会话查询
	return nil, ErrSessionNotFound
}

// Create 模拟创建登录会话。
func (*t03Sessions) Create(context.Context, string, Session, int64) error {
	// 1. T03 测试不涉及新登录会话创建
	return nil
}

// Delete 模拟删除当前登录会话。
func (*t03Sessions) Delete(context.Context, string, uint64) error {
	// 1. T03 测试不涉及当前会话退出
	return nil
}

// DeleteOtherSessions 记录其他设备会话收敛。
func (s *t03Sessions) DeleteOtherSessions(_ context.Context, token string, userID uint64) error {
	// 1. 记录收敛时保留的当前会话
	s.deletedToken, s.deletedUserID = token, userID
	return nil
}

// t03Storage 记录头像预签名和公开地址转换。
type t03Storage struct {
	presignKey string        // presignKey 是生成上传地址的对象 Key。
	publicURL  string        // publicURL 是头像完整公开地址。
	expires    time.Duration // expires 是头像上传地址有效期。
}

// PresignPut 返回固定头像上传地址。
func (s *t03Storage) PresignPut(_ context.Context, key string, expires time.Duration) (string, error) {
	// 1. 记录预签名请求参数
	s.presignKey, s.expires = key, expires
	return "signed", nil
}

// PublicURL 返回固定头像公开地址。
func (s *t03Storage) PublicURL(key string) string {
	// 1. 根据测试对象 Key 组装公开地址
	s.publicURL = "https://public/" + key
	return s.publicURL
}
