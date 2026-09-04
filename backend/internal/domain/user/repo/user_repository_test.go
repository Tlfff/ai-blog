package repo

import (
	"context"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

// TestUserRepositoryLifecycle 验证用户仓储在真实 SQL 引擎上的创建、查询和更新行为。
func TestUserRepositoryLifecycle(t *testing.T) {
	// 1. 建立隔离的内存数据库及与 users 基线一致的测试表
	engine, err := xorm.NewEngine("sqlite3", "file:user_repository?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("create database engine: %v", err)
	}
	defer func() {
		if closeErr := engine.Close(); closeErr != nil {
			t.Errorf("close database engine: %v", closeErr)
		}
	}()
	_, err = engine.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nickname TEXT NOT NULL UNIQUE,
			phone TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			avatar TEXT NULL,
			role INTEGER NOT NULL DEFAULT 1,
			created_time DATETIME NOT NULL,
			updated_time DATETIME NOT NULL,
			status INTEGER NOT NULL,
			last_login_ip TEXT NOT NULL DEFAULT '',
			last_login_time DATETIME NOT NULL
		)`)
	if err != nil {
		t.Fatalf("create users table: %v", err)
	}

	// 2. 创建正常用户并验证唯一性查询和资料读取
	repository := &UserRepository{client: engine}
	now := time.Now().Truncate(time.Second)
	created := &entity.User{
		Nickname:      "tester",
		Phone:         "13800138000",
		Password:      "pbkdf2$100000$salt$hash",
		Avatar:        user.DefaultAvatar,
		Role:          user.RoleUser,
		Status:        user.StatusNormal,
		CreatedTime:   now,
		UpdatedTime:   now,
		LastLoginTime: now,
	}
	if err := repository.Create(context.Background(), created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatal("Create() should write back generated user ID")
	}
	nicknameExists, err := repository.NicknameExists(context.Background(), created.Nickname, 0)
	if err != nil || !nicknameExists {
		t.Fatalf("NicknameExists() = %v, %v", nicknameExists, err)
	}
	phoneExists, err := repository.PhoneExists(context.Background(), created.Phone)
	if err != nil || !phoneExists {
		t.Fatalf("PhoneExists() = %v, %v", phoneExists, err)
	}
	found, err := repository.FindNormalByID(context.Background(), created.ID)
	if err != nil || found.Nickname != created.Nickname || found.Password != created.Password {
		t.Fatalf("FindNormalByID() = %#v, %v", found, err)
	}

	// 3. 更新公开资料并验证排除当前用户的唯一性语义
	found.Nickname = "updated"
	found.Avatar = "updated.png"
	if err := repository.UpdateProfile(context.Background(), found); err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	updated, err := repository.FindNormalByID(context.Background(), created.ID)
	if err != nil || updated.Nickname != "updated" || updated.Avatar != "updated.png" {
		t.Fatalf("updated profile = %#v, %v", updated, err)
	}
	existsForOther, err := repository.NicknameExists(context.Background(), "updated", created.ID)
	if err != nil || existsForOther {
		t.Fatalf("NicknameExists() excluding current user = %v, %v", existsForOther, err)
	}

	// 4. 登录查询支持手机号、昵称组合，并更新原始登录信息
	account, err := repository.FindNormalByAccount(context.Background(), created.Phone, "updated")
	if err != nil || account.ID != created.ID {
		t.Fatalf("FindNormalByAccount() = %#v, %v", account, err)
	}
	loginAt := now.Add(time.Hour)
	if err := repository.UpdateLogin(context.Background(), created.ID, "203.0.113.8", loginAt); err != nil {
		t.Fatalf("UpdateLogin() error = %v", err)
	}
	loggedIn, err := repository.FindNormalByID(context.Background(), created.ID)
	if err != nil || loggedIn.LastLoginIP != "203.0.113.8" || !loggedIn.LastLoginTime.Equal(loginAt) {
		t.Fatalf("login metadata = %#v, %v", loggedIn, err)
	}
}
