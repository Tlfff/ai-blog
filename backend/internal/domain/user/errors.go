package user

import "errors"

var (
	ErrNicknameExists             = errors.New("用户昵称已存在")     // ErrNicknameExists 表示昵称已被其他用户使用。
	ErrPhoneExists                = errors.New("手机号已存在")      // ErrPhoneExists 表示手机号已被其他用户使用。
	ErrUserNotFound               = errors.New("用户不存在")       // ErrUserNotFound 表示正常状态的用户不存在。
	ErrSessionNotFound            = errors.New("登录会话不存在")     // ErrSessionNotFound 表示访问 Token 没有有效会话。
	ErrPasswordChangeTokenInvalid = errors.New("改密凭证无效")      // ErrPasswordChangeTokenInvalid 表示凭证已过期、已消费或不属于当前用户。
	ErrInvalidAvatarObjectKey     = errors.New("头像对象不属于当前用户") // ErrInvalidAvatarObjectKey 表示头像对象 Key 不在当前用户目录。
	ErrInvalidPhone               = errors.New("手机号格式错误")     // ErrInvalidPhone 表示手机号不符合业务格式。
)

// ErrInvalidLogin 表示登录账号字段不符合协议约束。
var ErrInvalidLogin = errors.New("手机号或昵称至少提供一个，手机号只能为数字且昵称不能全为数字")

// ErrInvalidCredentials 表示账号不存在、已失效或密码错误。
var ErrInvalidCredentials = errors.New("手机号、昵称或密码错误")
