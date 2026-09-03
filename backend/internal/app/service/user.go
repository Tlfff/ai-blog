package service

import (
	"errors"
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
	codeNicknameExists  = 44020101 // codeNicknameExists 表示用户昵称冲突。
	codePhoneExists     = 44020102 // codePhoneExists 表示用户手机号冲突。
	codeUserNotFound    = 44020103 // codeUserNotFound 表示正常用户不存在。
	codeUnauthenticated = 44030101 // codeUnauthenticated 表示请求缺少登录身份。
)

// UserService 将用户 HTTP 协议转换为用户领域调用。
type UserService struct {
	useCase userdomain.UseCase // useCase 是用户上下文公开业务接口。
}

// NewUserServer 创建用户 HTTP 服务。
func NewUserServer(useCase *userdomain.Service) userapi.UserServiceHTTPServerController {
	// 1. 将用户领域能力暴露为生成的 HTTP Controller
	if useCase == nil {
		panic("用户 HTTP 服务缺少领域服务")
	}
	return &UserService{useCase: useCase}
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
	return profileReply(profile), nil
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
func profileReply(profile *entity.User) *userapi.ProfileReply {
	// 1. 只暴露当前用户接口约定的资料字段
	return &userapi.ProfileReply{
		Id:            profile.ID,
		Nickname:      profile.Nickname,
		Avatar:        profile.Avatar,
		Role:          int32(profile.Role),
		LastLoginTime: unixSeconds(profile.LastLoginTime),
		LastLoginIp:   profile.LastLoginIP,
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
	case errors.Is(err, userdomain.ErrUserNotFound):
		return errassets.NewError(codeUserNotFound, err.Error())
	default:
		return err
	}
}
