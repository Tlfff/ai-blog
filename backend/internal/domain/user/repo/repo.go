package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/repo/factory"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/repo/po"
	"github.com/go-sql-driver/mysql"
)

// UserRepository 使用 MySQL 实现用户领域仓储。
type UserRepository struct {
	client clients.MysqlClient // client 是博客业务数据库客户端。
}

// NewUserRepository 创建用户 MySQL 仓储。
func NewUserRepository(client clients.MysqlClient) *UserRepository {
	// 1. 启动阶段拒绝缺少数据库客户端的仓储
	if client == nil {
		panic("用户仓储缺少 MySQL 客户端")
	}
	return &UserRepository{client: client}
}

// NicknameExists 判断昵称是否已被其他账号使用。
func (r *UserRepository) NicknameExists(ctx context.Context, nickname string, excludeUserID uint64) (bool, error) {
	// 1. 按昵称查询，并在资料更新时排除当前用户
	session := r.client.Context(ctx).Table(new(po.User)).Where("nickname = ?", nickname)
	if excludeUserID > 0 {
		session = session.And("id <> ?", excludeUserID)
	}
	count, err := session.Count()
	return count > 0, err
}

// PhoneExists 判断手机号是否已被其他账号使用。
func (r *UserRepository) PhoneExists(ctx context.Context, phone string) (bool, error) {
	// 1. 使用手机号唯一索引检查账号是否存在
	count, err := r.client.Context(ctx).Table(new(po.User)).Where("phone = ?", phone).Count()
	return count > 0, err
}

// Create 创建用户并把数据库生成的 ID 回写到领域对象。
func (r *UserRepository) Create(ctx context.Context, entityUser *entity.User) error {
	// 1. 转换持久化对象并写入 users 表
	userPO := factory.UserToPO(entityUser)
	if _, err := r.client.Context(ctx).Insert(userPO); err != nil {
		return mapDuplicateError(err)
	}

	// 2. 回写数据库生成的用户标识
	entityUser.ID = userPO.ID
	return nil
}

// FindNormalByID 查询正常状态用户。
func (r *UserRepository) FindNormalByID(ctx context.Context, userID uint64) (*entity.User, error) {
	// 1. 只查询正常状态用户并转换为领域对象
	userPO := new(po.User)
	found, err := r.client.Context(ctx).
		Where("id = ? AND status = ?", userID, user.StatusNormal).
		Get(userPO)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, user.ErrUserNotFound
	}
	return factory.UserToEntity(userPO), nil
}

// UpdateProfile 更新正常状态用户的昵称和头像。
func (r *UserRepository) UpdateProfile(ctx context.Context, entityUser *entity.User) error {
	// 1. 只更新正常用户的公开资料字段
	userPO := factory.UserToPO(entityUser)
	rows, err := r.client.Context(ctx).
		Where("id = ? AND status = ?", entityUser.ID, user.StatusNormal).
		Cols("nickname", "avatar").
		Update(userPO)
	if err != nil {
		return mapDuplicateError(err)
	}
	if rows == 0 {
		return user.ErrUserNotFound
	}
	return nil
}

// mapDuplicateError 将 users 唯一索引冲突转换为稳定领域错误。
func mapDuplicateError(err error) error {
	// 1. 非 MySQL 唯一索引错误保持原始错误链
	var mysqlError *mysql.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return err
	}

	// 2. 根据稳定唯一索引名称映射用户领域错误
	message := strings.ToLower(mysqlError.Message)
	if strings.Contains(message, "uni_nickname") || strings.Contains(message, "nickname") {
		return user.ErrNicknameExists
	}
	if strings.Contains(message, "uni_phone") || strings.Contains(message, "phone") {
		return user.ErrPhoneExists
	}
	return err
}

// FindNormalByAccount 按非空手机号、昵称条件查询正常用户。
func (r *UserRepository) FindNormalByAccount(ctx context.Context, phone, nickname string) (*entity.User, error) {
	// 1. 限定正常状态，并按实际提供的账号字段组合查询
	userPO := new(po.User)
	session := r.client.Context(ctx).Where("status = ?", user.StatusNormal)
	if phone != "" {
		session = session.And("phone = ?", phone)
	}
	if nickname != "" {
		session = session.And("nickname = ?", nickname)
	}
	found, err := session.Get(userPO)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, user.ErrUserNotFound
	}
	return factory.UserToEntity(userPO), nil
}

// UpdateLogin 更新用户最后一次登录的原始 IP 和时间。
func (r *UserRepository) UpdateLogin(ctx context.Context, userID uint64, ip string, at time.Time) error {
	// 1. 只更新正常用户的登录来源和时间
	rows, err := r.client.Context(ctx).Where("id = ? AND status = ?", userID, user.StatusNormal).
		Cols("last_login_ip", "last_login_time").Update(&po.User{LastLoginIP: ip, LastLoginTime: at})
	if err != nil {
		return err
	}
	if rows == 0 {
		return user.ErrUserNotFound
	}
	return nil
}
