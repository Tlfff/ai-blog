package service

import (
	"errors"
	"strings"
	"time"

	userapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/user"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"codeup.aliyun.com/qimao/leo/lib/errassets"
	"github.com/gin-gonic/gin"
)

const (
	codeNicknameExists     = 44020101 // codeNicknameExists 表示用户昵称冲突。
	codePhoneExists        = 44020102 // codePhoneExists 表示用户手机号冲突。
	codeUserNotFound       = 44020103 // codeUserNotFound 表示正常用户不存在。
	codeUnauthenticated    = 44030101 // codeUnauthenticated 表示请求缺少登录身份。
	codeInvalidLogin       = 44030104 // codeInvalidLogin 表示登录请求账号字段不合法。
	codeInvalidPhone       = 44020104 // codeInvalidPhone 表示手机号格式不合法。
	codeInvalidAvatar      = 44020105 // codeInvalidAvatar 表示头像对象或扩展名不合法。
	codeInvalidChangeToken = 44030105 // codeInvalidChangeToken 表示改密凭证无效。
)

// UserService 将用户 HTTP 协议转换为用户领域调用。
type UserService struct {
	useCase           userdomain.UseCase          // useCase 是用户上下文公开业务接口。
	authUseCase       authUseCase                 // authUseCase 是登录和退出业务接口。
	regionResolver    userdomain.IPRegionResolver // regionResolver 将登录 IP 转换为资料地区文案。
	trustedProxyCIDRs []string                    // trustedProxyCIDRs 是允许透传客户端 IP 的代理网段。
}

// NewUserServer 创建用户 HTTP 服务。
func NewUserServer(useCase *userdomain.Service, resolver userdomain.IPRegionResolver, trustedProxyCIDRs []string) userapi.UserServiceHTTPServerController {
	// 1. 将用户领域能力暴露为生成的 HTTP Controller
	if useCase == nil || resolver == nil {
		panic("用户 HTTP 服务缺少领域服务或 IP 地区解析器")
	}
	return &UserService{useCase: useCase, authUseCase: useCase, regionResolver: resolver, trustedProxyCIDRs: trustedProxyCIDRs}
}

// Register 注册普通用户账号。
func (s *UserService) Register(ctx *gin.Context, request *userapi.RegisterRequest) (*userapi.EmptyReply, error) {
	// 1. 将协议请求转换为用户注册命令
	err := s.useCase.Register(ctx.Request.Context(), userdomain.RegisterCommand{
		Nickname: request.GetNickname(),
		Phone:    request.GetPhone(),
		Password: request.GetPassword(),
	})
	if err != nil {
		return nil, userHTTPError(err)
	}

	// 2. 设置兼容成功消息并交由生成代码渲染
	httpresponse.SetSuccess(ctx, "注册成功", true)
	return &userapi.EmptyReply{}, nil
}

// GetMyProfile 查询当前登录用户的完整资料。
func (s *UserService) GetMyProfile(ctx *gin.Context, _ *userapi.GetMyProfileRequest) (*userapi.ProfileReply, error) {
	// 1. 从认证中间件读取当前用户身份
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}

	// 2. 查询正常用户并转换为完整资料响应
	profile, err := s.useCase.GetProfile(ctx.Request.Context(), currentUser.ID)
	if err != nil {
		return nil, userHTTPError(err)
	}
	return s.profileReply(profile), nil
}

// GetPublicProfile 查询指定用户的公开资料。
func (s *UserService) GetPublicProfile(ctx *gin.Context, request *userapi.GetPublicProfileRequest) (*userapi.PublicProfileReply, error) {
	// 1. 查询正常用户资料
	profile, err := s.useCase.GetProfile(ctx.Request.Context(), request.GetUserId())
	if err != nil {
		return nil, userHTTPError(err)
	}

	// 2. 只组装公开字段，避免协议层泄漏账号信息
	return &userapi.PublicProfileReply{
		Id:       profile.ID,
		Nickname: profile.Nickname,
		Avatar:   profile.Avatar,
	}, nil
}

// UpdateMyProfile 修改当前登录用户的昵称和头像。
func (s *UserService) UpdateMyProfile(ctx *gin.Context, request *userapi.UpdateMyProfileRequest) (*userapi.EmptyReply, error) {
	// 1. 从认证中间件读取当前用户身份
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}

	// 2. 将协议请求转换为资料更新命令
	err := s.useCase.UpdateProfile(ctx.Request.Context(), userdomain.UpdateProfileCommand{
		UserID:   currentUser.ID,
		Nickname: request.GetNickname(),
		Avatar:   request.GetAvatar(),
	})
	if err != nil {
		return nil, userHTTPError(err)
	}

	// 3. 设置兼容成功消息并交由生成代码渲染
	httpresponse.SetSuccess(ctx, "个人资料修改成功", true)
	return &userapi.EmptyReply{}, nil
}

// profileReply 将用户领域对象转换为当前用户资料响应。
func (s *UserService) profileReply(profile *entity.User) *userapi.ProfileReply {
	// 1. 只暴露当前用户接口约定的资料字段
	region := s.regionResolver.Resolve(profile.LastLoginIP)
	return &userapi.ProfileReply{
		Id:            profile.ID,
		Nickname:      profile.Nickname,
		Avatar:        profile.Avatar,
		Role:          int32(profile.Role),
		LastLoginTime: unixSeconds(profile.LastLoginTime),
		LastLoginIp:   region,
	}
}

// unixSeconds 将数据库时间转换为接口约定的 Unix 秒。
func unixSeconds(value time.Time) int64 {
	// 1. 零时间保持为 0，其他时间转换为 Unix 秒
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

// userHTTPError 将用户领域错误转换为稳定业务码。
func userHTTPError(err error) error {
	// 1. 将可预期领域错误映射为用户模块稳定业务码
	switch {
	case errors.Is(err, userdomain.ErrNicknameExists):
		return errassets.NewError(codeNicknameExists, err.Error())
	case errors.Is(err, userdomain.ErrPhoneExists):
		return errassets.NewError(codePhoneExists, err.Error())
	case errors.Is(err, userdomain.ErrInvalidCredentials):
		return errassets.NewError(codeUnauthenticated, err.Error())
	case errors.Is(err, userdomain.ErrInvalidPhone):
		return errassets.NewError(codeInvalidPhone, err.Error())
	case errors.Is(err, userdomain.ErrInvalidAvatarObjectKey):
		return errassets.NewError(codeInvalidAvatar, err.Error())
	case errors.Is(err, userdomain.ErrPasswordChangeTokenInvalid):
		return errassets.NewError(codeInvalidChangeToken, err.Error())
	case errors.Is(err, userdomain.ErrInvalidLogin):
		return errassets.NewError(codeInvalidLogin, err.Error())
	case errors.Is(err, userdomain.ErrUserNotFound):
		return errassets.NewError(codeUserNotFound, err.Error())
	default:
		return err
	}
}

// VerifyOldPassword 验证旧密码并返回一次性改密凭证。
func (s *UserService) VerifyOldPassword(ctx *gin.Context, request *userapi.VerifyOldPasswordRequest) (*userapi.PasswordChangeTokenReply, error) {
	// 1. 读取认证身份并调用领域服务验证旧密码
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}
	token, err := s.useCase.VerifyOldPassword(ctx.Request.Context(), currentUser.ID, request.GetOldPassword())
	if err != nil {
		return nil, userHTTPError(err)
	}
	// 2. 返回一次性改密凭证
	return &userapi.PasswordChangeTokenReply{ChangeToken: token}, nil
}

// ChangePassword 使用一次性凭证修改密码并收敛其他设备会话。
func (s *UserService) ChangePassword(ctx *gin.Context, request *userapi.ChangePasswordRequest) (*userapi.EmptyReply, error) {
	// 1. 校验当前身份和当前设备 Token
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}
	token, valid := parseBearerToken(ctx.GetHeader("Authorization"))
	if !valid {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}
	// 2. 消费凭证、修改密码并收敛其他设备会话
	err := s.useCase.ChangePassword(ctx.Request.Context(), userdomain.ChangePasswordCommand{UserID: currentUser.ID, CurrentToken: token, ChangeToken: request.GetChangeToken(), NewPassword: request.GetNewPassword()})
	if err != nil {
		return nil, userHTTPError(err)
	}
	// 3. 返回兼容成功消息和 null 数据
	httpresponse.SetSuccess(ctx, "密码修改成功", true)
	return &userapi.EmptyReply{}, nil
}

// UpdateMyAccount 修改当前用户手机号。
func (s *UserService) UpdateMyAccount(ctx *gin.Context, request *userapi.UpdateMyAccountRequest) (*userapi.EmptyReply, error) {
	// 1. 读取当前用户并交由领域服务校验手机号唯一性
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}
	if err := s.useCase.UpdatePhone(ctx.Request.Context(), userdomain.UpdatePhoneCommand{UserID: currentUser.ID, Phone: strings.TrimSpace(request.GetPhone())}); err != nil {
		return nil, userHTTPError(err)
	}
	// 2. 返回兼容成功消息和 null 数据
	httpresponse.SetSuccess(ctx, "电话修改成功", true)
	return &userapi.EmptyReply{}, nil
}

// GetAvatarUploadURL 获取当前用户头像直传凭证。
func (s *UserService) GetAvatarUploadURL(ctx *gin.Context, request *userapi.GetAvatarUploadURLRequest) (*userapi.AvatarUploadReply, error) {
	// 1. 读取当前用户并请求领域服务生成 MinIO 预签名地址
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}
	result, err := s.useCase.GetAvatarUploadURL(ctx.Request.Context(), currentUser.ID, request.GetFileExt())
	if err != nil {
		return nil, userHTTPError(err)
	}
	// 2. 返回上传地址和稳定对象 Key
	return &userapi.AvatarUploadReply{UploadUrl: result.UploadURL, ObjectKey: result.ObjectKey}, nil
}

// ConfirmAvatar 确认头像对象并返回公开地址。
func (s *UserService) ConfirmAvatar(ctx *gin.Context, request *userapi.ConfirmAvatarRequest) (*userapi.AvatarUploadReply, error) {
	// 1. 读取当前用户并由领域服务校验头像对象归属后保存
	currentUser, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(codeUnauthenticated, "未登录")
	}
	avatarURL, err := s.useCase.ConfirmAvatar(ctx.Request.Context(), currentUser.ID, request.GetObjectKey())
	if err != nil {
		return nil, userHTTPError(err)
	}
	// 2. 返回头像公开访问地址
	return &userapi.AvatarUploadReply{AvatarUrl: avatarURL}, nil
}
