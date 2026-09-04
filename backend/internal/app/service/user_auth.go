package service

import (
	"context"
	"strings"

	userapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/user"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/requestip"
	"codeup.aliyun.com/qimao/leo/lib/errassets"
	"github.com/gin-gonic/gin"
)

// LoginRequest 是登录协议请求。
type LoginRequest = userapi.LoginRequest

// LoginReply 是登录协议响应。
type LoginReply = userapi.LoginReply

// LogoutRequest 是退出协议请求。
type LogoutRequest = userapi.LogoutRequest

// authUseCase 定义应用层所需的登录和退出能力。
type authUseCase interface {
	// Login 创建用户登录会话。
	Login(context.Context, userdomain.LoginCommand) (*userdomain.LoginResult, error)
	// Logout 撤销当前访问 Token。
	Logout(context.Context, string) error
}

// Login 将协议请求转换为登录领域命令并返回访问 Token。
func (s *UserService) Login(ctx *gin.Context, request *LoginRequest) (*LoginReply, error) {
	// 1. 清理账号输入并交由领域服务统一校验业务约束
	if s.authUseCase == nil {
		panic("用户登录服务缺少认证用例")
	}
	result, err := s.authUseCase.Login(ctx.Request.Context(), userdomain.LoginCommand{
		Phone:      strings.TrimSpace(request.GetPhone()),
		Nickname:   strings.TrimSpace(request.GetNickname()),
		Password:   request.GetPassword(),
		RememberMe: request.GetRememberMe(),
		Device:     strings.TrimSpace(request.GetDevice()),
		ClientIP:   requestip.FromRequest(ctx.Request, s.trustedProxyCIDRs),
	})
	if err != nil {
		return nil, userHTTPError(err)
	}

	// 2. 返回当前设备的访问 Token
	return &LoginReply{AccessToken: result.AccessToken}, nil
}

// Logout 只撤销当前请求携带的设备会话。
func (s *UserService) Logout(ctx *gin.Context, _ *LogoutRequest) (*userapi.EmptyReply, error) {
	// 1. 从请求头取得当前设备携带的 Token
	if s.authUseCase == nil {
		panic("用户退出服务缺少认证用例")
	}
	token, ok := parseBearerToken(ctx.GetHeader("Authorization"))
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}

	// 2. 仅撤销当前 Token 对应的登录会话
	if err := s.authUseCase.Logout(ctx.Request.Context(), token); err != nil {
		return nil, userHTTPError(err)
	}

	// 3. 返回兼容成功消息和 null 数据
	httpresponse.SetSuccess(ctx, "退出成功", true)
	return &userapi.EmptyReply{}, nil
}

// parseBearerToken 解析严格的 Bearer Token 头部格式。
func parseBearerToken(value string) (string, bool) {
	// 1. Header 必须由 Bearer 类型和非空 Token 两部分组成
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], parts[1] != ""
}
