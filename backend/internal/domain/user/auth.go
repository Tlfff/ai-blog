package user

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
)

// LoginCommand 是登录所需的领域输入。
type LoginCommand struct {
	Phone      string // Phone 是登录手机号。
	Nickname   string // Nickname 是登录昵称。
	Password   string // Password 是待校验的明文密码。
	RememberMe bool   // RememberMe 表示是否延长会话到 30 天。
	Device     string // Device 是当前登录设备标识。
	ClientIP   string // ClientIP 是按受信代理规则取得的原始客户端地址。
}

// LoginResult 是登录成功后返回的会话凭证。
type LoginResult struct {
	AccessToken string // AccessToken 是安全随机生成的访问凭证。
	ExpiresIn   int64  // ExpiresIn 是会话有效期，单位为秒。
}

// AuthRepository 定义登录所需的用户读写能力。
type AuthRepository interface {
	// FindNormalByAccount 按非空手机号、昵称条件查询正常用户。
	FindNormalByAccount(ctx context.Context, phone, nickname string) (*entity.User, error)
	// UpdateLogin 更新用户最后登录 IP 和时间。
	UpdateLogin(ctx context.Context, userID uint64, ip string, at time.Time) error
}
