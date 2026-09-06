package service

import (
	"context"
	"errors"

	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OpenUserGRPCService 将开放 gRPC 用户协议适配到用户上下文公开查询接口。
type OpenUserGRPCService struct {
	blogopenv1.UnimplementedUserServiceServer                             // UnimplementedUserServiceServer 保持生成接口的前向兼容。
	users                                     userdomain.QueryUseCase     // users 是用户上下文公开的只读应用接口。
	regionResolver                            userdomain.IPRegionResolver // regionResolver 将原始登录 IP 转为脱敏地区。
}

// NewOpenUserGRPCServer 创建开放用户查询 gRPC 服务。
func NewOpenUserGRPCServer(users userdomain.QueryUseCase, regionResolver userdomain.IPRegionResolver) blogopenv1.UserServiceServer {
	// 1. 启动阶段拒绝缺少用户公开接口或地区解析器的服务
	if users == nil || regionResolver == nil {
		panic("开放用户 gRPC 服务缺少必要依赖")
	}

	// 2. 仅保存公开接口，不向 gRPC 层注入仓储或数据库客户端
	return &OpenUserGRPCService{users: users, regionResolver: regionResolver}
}

// GetUserBasicInfo 返回内部服务可见的用户基本信息。
func (s *OpenUserGRPCService) GetUserBasicInfo(ctx context.Context, request *blogopenv1.GetUserInfoRequest) (*blogopenv1.UserBasicInfoReply, error) {
	// 1. 通过用户上下文公开接口校验并查询正常用户
	profile, err := s.getUserProfile(ctx, request)
	if err != nil {
		return nil, err
	}

	// 2. 转换时间和登录地区后组装内部响应
	return &blogopenv1.UserBasicInfoReply{
		UserId:        profile.ID,
		Nickname:      profile.Nickname,
		Avatar:        profile.Avatar,
		LastLoginTime: unixSeconds(profile.LastLoginTime),
		LastLoginIp:   s.regionResolver.Resolve(profile.LastLoginIP),
	}, nil
}

// GetPublicUserInfo 仅返回外部合作方可见的公开资料。
func (s *OpenUserGRPCService) GetPublicUserInfo(ctx context.Context, request *blogopenv1.GetUserInfoRequest) (*blogopenv1.PublicUserInfoReply, error) {
	// 1. 通过用户上下文公开接口校验并查询正常用户
	profile, err := s.getUserProfile(ctx, request)
	if err != nil {
		return nil, err
	}

	// 2. 仅组装头像和昵称等公开字段
	return &blogopenv1.PublicUserInfoReply{Id: profile.ID, Avatar: profile.Avatar, Nickname: profile.Nickname}, nil
}

// getUserProfile 校验协议参数并通过用户上下文公开接口查询正常用户。
func (s *OpenUserGRPCService) getUserProfile(ctx context.Context, request *blogopenv1.GetUserInfoRequest) (*entity.User, error) {
	// 1. 在调用用户上下文前校验必要字段
	if request == nil || request.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id 必须大于 0")
	}

	// 2. 将领域错误转换为稳定且不泄漏细节的 gRPC Code
	profile, err := s.users.GetProfile(ctx, request.GetUserId())
	if err != nil {
		return nil, userGRPCError(err)
	}
	if profile == nil {
		return nil, status.Error(codes.Internal, "查询用户失败")
	}
	return profile, nil
}

// userGRPCError 将用户领域错误映射为稳定且不泄漏内部细节的 gRPC Code。
func userGRPCError(err error) error {
	// 1. 将可预期的用户不存在映射为 NotFound
	if errors.Is(err, userdomain.ErrUserNotFound) {
		return status.Error(codes.NotFound, "用户不存在")
	}

	// 2. 其他依赖错误统一隐藏为 Internal
	return status.Error(codes.Internal, "查询用户失败")
}
