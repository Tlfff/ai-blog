package user

import (
	"context"
	"errors"
	"testing"

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

type fakePasswordHasher struct {
	input  string // input 记录收到的明文密码。
	hashed string // hashed 是预设密码摘要。
	err    error  // err 是预设摘要错误。
}

// Hash 记录明文密码并返回测试预设摘要。
func (f *fakePasswordHasher) Hash(password string) (string, error) {
	f.input = password
	return f.hashed, f.err
}
