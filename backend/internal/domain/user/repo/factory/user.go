// Package factory 负责用户领域对象与持久化对象之间的转换。
package factory

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/repo/po"
)

// UserToPO 将用户领域对象转换为持久化对象。
func UserToPO(user *entity.User) *po.User {
	var avatar *string
	if user.Avatar != "" {
		avatar = &user.Avatar
	}
	return &po.User{
		ID:            user.ID,
		Nickname:      user.Nickname,
		Phone:         user.Phone,
		Password:      user.Password,
		Avatar:        avatar,
		Role:          user.Role,
		CreatedTime:   user.CreatedTime,
		UpdatedTime:   user.UpdatedTime,
		Status:        user.Status,
		LastLoginIP:   user.LastLoginIP,
		LastLoginTime: user.LastLoginTime,
	}
}

// UserToEntity 将持久化对象转换为用户领域对象。
func UserToEntity(user *po.User) *entity.User {
	avatar := ""
	if user.Avatar != nil {
		avatar = *user.Avatar
	}
	return &entity.User{
		ID:            user.ID,
		Nickname:      user.Nickname,
		Phone:         user.Phone,
		Password:      user.Password,
		Avatar:        avatar,
		Role:          user.Role,
		CreatedTime:   user.CreatedTime,
		UpdatedTime:   user.UpdatedTime,
		Status:        user.Status,
		LastLoginIP:   user.LastLoginIP,
		LastLoginTime: user.LastLoginTime,
	}
}
