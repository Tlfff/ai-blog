package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
)

// TestServiceRegister 验证注册成功及昵称、手机号冲突规则。
func TestServiceRegister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string          // name 是测试场景名称。
		repo           *fakeRepository // repo 是场景使用的用户仓储。
		wantErr        error           // wantErr 是预期领域错误。
		wantCreateCall bool            // wantCreateCall 表示是否应创建用户。
	}{
		{
			name:           "注册成功",
			repo:           &fakeRepository{},
			wantCreateCall: true,
		},
		{
			name:    "昵称已存在",
			repo:    &fakeRepository{nicknameExists: true},
			wantErr: ErrNicknameExists,
		},
		{
			name:    "手机号已存在",
			repo:    &fakeRepository{phoneExists: true},
			wantErr: ErrPhoneExists,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hasher := &fakePasswordHasher{hashed: "pbkdf2$100000$salt$hash"}
			service := NewService(tt.repo, hasher)
			err := service.Register(context.Background(), RegisterCommand{
				Nickname: "tester",
				Phone:    "13800138000",
				Password: "secret1",
			})

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Register() error = %v, want %v", err, tt.wantErr)
			}
			if tt.repo.createCalled != tt.wantCreateCall {
				t.Fatalf("Create() called = %v, want %v", tt.repo.createCalled, tt.wantCreateCall)
			}
			if !tt.wantCreateCall {
				return
			}
			if hasher.input != "secret1" {
				t.Fatalf("Hash() input = %q, want secret1", hasher.input)
			}
			if tt.repo.created.Password != hasher.hashed {
				t.Fatalf("created password = %q, want hashed password", tt.repo.created.Password)
			}
			if tt.repo.created.Role != RoleUser || tt.repo.created.Status != StatusNormal {
				t.Fatalf("created defaults = role %d status %d", tt.repo.created.Role, tt.repo.created.Status)
			}
			if tt.repo.created.Avatar != DefaultAvatar {
				t.Fatalf("created avatar = %q, want %q", tt.repo.created.Avatar, DefaultAvatar)
			}
			if tt.repo.created.CreatedTime.IsZero() || tt.repo.created.LastLoginTime.IsZero() {
				t.Fatal("registration times should be initialized")
			}
		})
	}
}

// TestServiceGetProfileOnlyReturnsNormalUser 验证资料查询只返回正常状态用户。
func TestServiceGetProfileOnlyReturnsNormalUser(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{found: &entity.User{ID: 7, Status: StatusNormal}}
	service := NewService(repository, &fakePasswordHasher{})

	got, err := service.GetProfile(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if got.ID != 7 {
		t.Fatalf("GetProfile() ID = %d, want 7", got.ID)
	}

	repository.found = nil
	_, err = service.GetProfile(context.Background(), 8)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("GetProfile() error = %v, want %v", err, ErrUserNotFound)
	}

	repository.found = &entity.User{ID: 9, Status: StatusDeleted}
	_, err = service.GetProfile(context.Background(), 9)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("GetProfile() deleted user error = %v, want %v", err, ErrUserNotFound)
	}
}

// TestServiceUpdateProfile 验证资料更新和昵称唯一性规则。
func TestServiceUpdateProfile(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{found: &entity.User{ID: 9, Nickname: "old", Status: StatusNormal}}
	service := NewService(repository, &fakePasswordHasher{})

	err := service.UpdateProfile(context.Background(), UpdateProfileCommand{
		UserID:   9,
		Nickname: "new",
		Avatar:   "avatar.png",
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if repository.updated == nil || repository.updated.Nickname != "new" || repository.updated.Avatar != "avatar.png" {
		t.Fatalf("updated user = %#v", repository.updated)
	}

	repository.nicknameExists = true
	err = service.UpdateProfile(context.Background(), UpdateProfileCommand{UserID: 9, Nickname: "used"})
	if !errors.Is(err, ErrNicknameExists) {
		t.Fatalf("UpdateProfile() error = %v, want %v", err, ErrNicknameExists)
	}
}

type fakeRepository struct {
	nicknameExists bool         // nicknameExists 控制昵称唯一性查询结果。
	phoneExists    bool         // phoneExists 控制手机号唯一性查询结果。
	found          *entity.User // found 是资料查询返回的用户。
	created        *entity.User // created 记录仓储收到的新用户。
	updated        *entity.User // updated 记录仓储收到的资料变更。
	createCalled   bool         // createCalled 记录是否调用创建方法。
}

// NicknameExists 返回测试预设的昵称唯一性结果。
func (f *fakeRepository) NicknameExists(context.Context, string, uint64) (bool, error) {
	return f.nicknameExists, nil
}

// PhoneExists 返回测试预设的手机号唯一性结果。
func (f *fakeRepository) PhoneExists(context.Context, string) (bool, error) {
	return f.phoneExists, nil
}

// Create 记录待创建用户。
func (f *fakeRepository) Create(_ context.Context, user *entity.User) error {
	f.createCalled = true
	clone := *user
	f.created = &clone
	return nil
}

// FindNormalByID 返回测试预设的正常用户。
func (f *fakeRepository) FindNormalByID(context.Context, uint64) (*entity.User, error) {
	if f.found == nil {
		return nil, ErrUserNotFound
	}
	clone := *f.found
	return &clone, nil
}

// UpdateProfile 记录待更新用户资料。
func (f *fakeRepository) UpdateProfile(_ context.Context, user *entity.User) error {
	clone := *user
	f.updated = &clone
	return nil
}

// UpdatePassword 模拟更新密码摘要。
func (f *fakeRepository) UpdatePassword(context.Context, uint64, string) error {
	// 1. 通用用户测试默认更新成功
	return nil
}

// UpdatePhone 模拟更新手机号。
func (f *fakeRepository) UpdatePhone(context.Context, uint64, string) error {
	// 1. 通用用户测试默认更新成功
	return nil
}

// UpdateAvatar 模拟更新头像对象 Key。
func (f *fakeRepository) UpdateAvatar(context.Context, uint64, string) error {
	// 1. 通用用户测试默认更新成功
	return nil
}

// UpdatePasswordWithCleanupTask 模拟密码和会话补偿任务事务写入。
func (f *fakeRepository) UpdatePasswordWithCleanupTask(context.Context, uint64, string, string) error {
	// 1. 通用用户测试默认事务写入成功
	return nil
}

// ListSessionCleanupTasks 返回空的待处理补偿任务。
func (f *fakeRepository) ListSessionCleanupTasks(context.Context, int) ([]*SessionCleanupTask, error) {
	// 1. 通用用户测试默认没有补偿任务
	return nil, nil
}

// CompleteSessionCleanupTask 模拟完成补偿任务。
func (f *fakeRepository) CompleteSessionCleanupTask(context.Context, uint64) error {
	// 1. 通用用户测试默认完成成功
	return nil
}

// CompleteSessionCleanupTaskForSession 模拟完成当前会话补偿任务。
func (f *fakeRepository) CompleteSessionCleanupTaskForSession(context.Context, uint64, string) error {
	// 1. 通用用户测试默认完成成功
	return nil
}

type fakePasswordHasher struct {
	input    string // input 记录收到的明文密码。
	hashed   string // hashed 是预设密码摘要。
	err      error  // err 是预设摘要错误。
	compared string // compared 记录执行过密码比较。
}

// Hash 记录明文密码并返回测试预设摘要。
func (f *fakePasswordHasher) Hash(password string) (string, error) {
	f.input = password
	return f.hashed, f.err
}

// Compare 返回测试预设的密码比较结果。
func (f *fakePasswordHasher) Compare(string, string) (bool, error) {
	// 1. 记录比较已执行并返回预设结果
	f.compared = "compared"
	return true, f.err
}

// TestServiceLoginCreatesIndependentSessions 验证手机号、昵称登录及两种会话有效期。
func TestServiceLoginCreatesIndependentSessions(t *testing.T) {
	t.Parallel()
	password := (&PBKDF2PasswordHasher{})
	hash, err := password.Hash("secret1")
	if err != nil {
		t.Fatal(err)
	}
	repository := &fakeAuthRepository{user: &entity.User{ID: 7, Role: RoleUser, Status: StatusNormal, Password: hash}}
	sessions := &fakeSessionManager{}
	service := NewServiceWithSession(repository, repository, password, sessions)

	first, err := service.Login(context.Background(), LoginCommand{Phone: "13800138000", Password: "secret1", Device: "web", ClientIP: "203.0.113.9"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	second, err := service.Login(context.Background(), LoginCommand{Nickname: "tester", Password: "secret1", RememberMe: true, Device: "ios", ClientIP: "203.0.113.10"})
	if err != nil {
		t.Fatalf("Login() remember error = %v", err)
	}
	if first.AccessToken == second.AccessToken || len(first.AccessToken) != 64 || len(second.AccessToken) != 64 {
		t.Fatalf("tokens are not independent secure values: %q %q", first.AccessToken, second.AccessToken)
	}
	if first.ExpiresIn != int64((7*24*time.Hour).Seconds()) || second.ExpiresIn != int64((30*24*time.Hour).Seconds()) {
		t.Fatalf("expires = %d/%d", first.ExpiresIn, second.ExpiresIn)
	}
	if len(sessions.created) != 2 || sessions.created[0].ttl != first.ExpiresIn || sessions.created[1].ttl != second.ExpiresIn {
		t.Fatalf("created sessions = %#v", sessions.created)
	}
	if sessions.created[0].session.Device != "web" || sessions.created[1].session.Device != "ios" || sessions.created[1].session.LoginIP != "203.0.113.10" || sessions.created[1].session.LoginTime == 0 {
		t.Fatalf("session metadata = %#v", sessions.created)
	}
	if repository.lastIP != "203.0.113.10" || repository.lastLoginAt.IsZero() {
		t.Fatalf("login metadata = %q/%v", repository.lastIP, repository.lastLoginAt)
	}
}

// TestServiceLoginValidationAndCredentialPrivacy 验证账号字段约束和登录防枚举行为。
func TestServiceLoginValidationAndCredentialPrivacy(t *testing.T) {
	t.Parallel()

	password := &fakePasswordHasher{}
	repository := &fakeAuthRepository{}
	sessions := &fakeSessionManager{}
	service := NewServiceWithSession(repository, repository, password, sessions)
	tests := []struct {
		name    string       // name 是测试场景名称。
		command LoginCommand // command 是登录命令。
		want    error        // want 是预期领域错误。
	}{
		{name: "账号均为空", command: LoginCommand{Password: "secret1"}, want: ErrInvalidLogin},
		{name: "手机号非数字", command: LoginCommand{Phone: "abc", Password: "secret1"}, want: ErrInvalidLogin},
		{name: "昵称全数字", command: LoginCommand{Nickname: "123", Password: "secret1"}, want: ErrInvalidLogin},
		{name: "账号不存在", command: LoginCommand{Phone: "13800138000", Password: "secret1"}, want: ErrInvalidCredentials},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Login(context.Background(), test.command); !errors.Is(err, test.want) {
				t.Fatalf("Login() error = %v, want %v", err, test.want)
			}
		})
	}
}

// TestServiceLoginMissingAccountStillVerifiesPassword 验证账号不存在时仍执行等价密码校验。
func TestServiceLoginMissingAccountStillVerifiesPassword(t *testing.T) {
	t.Parallel()
	hasher := &fakePasswordHasher{}
	repository := &fakeAuthRepository{}
	service := NewServiceWithSession(repository, repository, hasher, &fakeSessionManager{})
	_, err := service.Login(context.Background(), LoginCommand{Phone: "13800138000", Password: "secret1"})
	if !errors.Is(err, ErrInvalidCredentials) || hasher.compared == "" {
		t.Fatalf("Login() error = %v compared = %q", err, hasher.compared)
	}
}

// TestServiceLogoutOnlyDeletesCurrentSession 验证退出只删除当前 Token。
func TestServiceLogoutOnlyDeletesCurrentSession(t *testing.T) {
	sessions := &fakeSessionManager{byToken: map[string]*Session{
		"current": {UserID: 7, Role: RoleUser, Device: "web"},
		"other":   {UserID: 7, Role: RoleUser, Device: "ios"},
	}}
	repository := &fakeAuthRepository{}
	service := NewServiceWithSession(repository, repository, &fakePasswordHasher{}, sessions)
	if err := service.Logout(context.Background(), "current"); err != nil {
		t.Fatal(err)
	}
	if sessions.deletedToken != "current" || sessions.deletedUserID != 7 {
		t.Fatalf("delete = %q/%d", sessions.deletedToken, sessions.deletedUserID)
	}
	if _, exists := sessions.byToken["other"]; !exists {
		t.Fatal("Logout() should preserve other device session")
	}
}

type fakeAuthRepository struct {
	fakeRepository              // fakeRepository 提供资料相关仓储能力。
	user           *entity.User // user 是预设登录用户。
	lastIP         string       // lastIP 记录最后登录 IP。
	lastLoginAt    time.Time    // lastLoginAt 记录最后登录时间。
}

// FindNormalByAccount 返回测试预设的登录用户。
func (f *fakeAuthRepository) FindNormalByAccount(context.Context, string, string) (*entity.User, error) {
	// 1. 返回预设用户或账号不存在错误
	if f.user == nil {
		return nil, ErrUserNotFound
	}
	return f.user, nil
}

// UpdateLogin 记录测试中的最后登录信息。
func (f *fakeAuthRepository) UpdateLogin(_ context.Context, _ uint64, ip string, at time.Time) error {
	// 1. 记录登录来源和时间供测试断言
	f.lastIP, f.lastLoginAt = ip, at
	return nil
}

type fakeSessionManager struct {
	created []struct {
		token   string  // token 是创建的访问 Token。
		ttl     int64   // ttl 是会话有效期秒数。
		session Session // session 是写入的会话内容。
	}
	found         *Session            // found 是预设查询会话。
	byToken       map[string]*Session // byToken 保存多设备测试会话。
	deletedToken  string              // deletedToken 记录删除的 Token。
	deletedUserID uint64              // deletedUserID 记录 Token 所属用户。
}

// FindByToken 返回测试预设的会话。
func (f *fakeSessionManager) FindByToken(_ context.Context, token string) (*Session, error) {
	// 1. 多设备场景按 Token 返回对应会话
	if f.byToken != nil {
		session, exists := f.byToken[token]
		if !exists {
			return nil, ErrSessionNotFound
		}
		return session, nil
	}
	// 2. 其他场景返回单个预设会话
	if f.found == nil {
		return nil, ErrSessionNotFound
	}
	return f.found, nil
}

// Create 记录待创建的登录会话。
func (f *fakeSessionManager) Create(_ context.Context, token string, session Session, ttl int64) error {
	// 1. 记录创建的 Token、会话内容和有效期
	f.created = append(f.created, struct {
		token   string
		ttl     int64
		session Session
	}{token, ttl, session})
	return nil
}

// Delete 记录待删除的当前设备会话。
func (f *fakeSessionManager) Delete(_ context.Context, token string, userID uint64) error {
	// 1. 只删除指定 Token 并保留其他设备会话
	f.deletedToken, f.deletedUserID = token, userID
	delete(f.byToken, token)
	return nil
}

// DeleteOtherSessions 保持既有会话测试的当前设备兼容行为。
func (f *fakeSessionManager) DeleteOtherSessions(_ context.Context, token string, userID uint64) error {
	// 1. 记录密码修改后需要保留的当前会话
	f.deletedToken, f.deletedUserID = token, userID
	return nil
}
