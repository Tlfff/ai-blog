package user

import "errors"

var (
	ErrNicknameExists  = errors.New("用户昵称已存在") // ErrNicknameExists 表示昵称已被其他用户使用。
	ErrPhoneExists     = errors.New("手机号已存在")  // ErrPhoneExists 表示手机号已被其他用户使用。
	ErrUserNotFound    = errors.New("用户不存在")   // ErrUserNotFound 表示正常状态的用户不存在。
	ErrSessionNotFound = errors.New("登录会话不存在") // ErrSessionNotFound 表示访问 Token 没有有效会话。
)
