package service

import (
	"context"
	"errors"
	"testing"
	"time"

	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeUserQuery 记录 gRPC 应用服务对用户公开查询接口的调用。
type fakeUserQuery struct {
	profile *entity.User // profile 是测试返回的正常用户资料。
	err     error        // err 是测试注入的用户查询错误。
	calls   int          // calls 是公开查询接口调用次数。
	userID  uint64       // userID 是最近一次查询的用户标识。
}

// GetProfile 返回预设用户资料并记录调用参数。
func (f *fakeUserQuery) GetProfile(_ context.Context, userID uint64) (*entity.User, error) {
	// 1. 记录应用服务是否只通过公开接口查询用户
	f.calls++
	f.userID = userID
	return f.profile, f.err
}

// fakeGRPCRegionResolver 返回固定脱敏地区。
type fakeGRPCRegionResolver struct {
	result string // result 是预设的地区文案。
}

// Resolve 返回预设地区文案。
func (f fakeGRPCRegionResolver) Resolve(string) string {
	// 1. 避免测试依赖真实 XDB 数据
	return f.result
}

// TestOpenUserGRPCServiceQueriesOnlyUserPublicInterface 验证两个 RPC 只依赖用户公开查询接口。
func TestOpenUserGRPCServiceQueriesOnlyUserPublicInterface(t *testing.T) {
	// 1. 准备包含私有字段的用户资料和脱敏地区替身
	loginAt := time.Date(2026, 9, 6, 11, 30, 0, 0, time.UTC)
	query := &fakeUserQuery{profile: &entity.User{
		ID: 42, Nickname: "tester", Avatar: "/avatar.png", Phone: "13800000000", Role: userdomain.RoleAdmin,
		LastLoginIP: "203.0.113.8", LastLoginTime: loginAt, Status: userdomain.StatusNormal,
	}}

	// 2. 调用内部 RPC 并验证登录信息经过协议转换
	service := NewOpenUserGRPCServer(query, fakeGRPCRegionResolver{result: "浙江"})
	basic, err := service.GetUserBasicInfo(context.Background(), &blogopenv1.GetUserInfoRequest{UserId: 42})
	if err != nil {
		t.Fatalf("GetUserBasicInfo() error = %v", err)
	}
	if query.calls != 1 || query.userID != 42 {
		t.Fatalf("query calls = %d, userID = %d", query.calls, query.userID)
	}
	if basic.GetUserId() != 42 || basic.GetNickname() != "tester" || basic.GetAvatar() != "/avatar.png" || basic.GetLastLoginTime() != loginAt.Unix() || basic.GetLastLoginIp() != "浙江" {
		t.Fatalf("basic reply = %#v", basic)
	}

	// 3. 调用外部 RPC 并验证响应仅包含公开字段
	public, err := service.GetPublicUserInfo(context.Background(), &blogopenv1.GetUserInfoRequest{UserId: 42})
	if err != nil {
		t.Fatalf("GetPublicUserInfo() error = %v", err)
	}
	if public.GetId() != 42 || public.GetNickname() != "tester" || public.GetAvatar() != "/avatar.png" {
		t.Fatalf("public reply = %#v", public)
	}
	if query.calls != 2 {
		t.Fatalf("query calls after public RPC = %d", query.calls)
	}
}

// TestOpenUserGRPCServiceMapsValidationAndDomainErrors 验证协议与领域错误使用标准 gRPC Code。
func TestOpenUserGRPCServiceMapsValidationAndDomainErrors(t *testing.T) {
	// 1. 覆盖参数、用户不存在和内部依赖错误
	tests := []struct {
		name    string                         // name 是测试场景名称。
		request *blogopenv1.GetUserInfoRequest // request 是待验证的协议请求。
		err     error                          // err 是用户公开接口返回的错误。
		want    codes.Code                     // want 是期望的标准 gRPC Code。
	}{
		{name: "无效用户 ID", request: &blogopenv1.GetUserInfoRequest{}, want: codes.InvalidArgument},
		{name: "用户不存在", request: &blogopenv1.GetUserInfoRequest{UserId: 1}, err: userdomain.ErrUserNotFound, want: codes.NotFound},
		{name: "查询返回空用户", request: &blogopenv1.GetUserInfoRequest{UserId: 1}, want: codes.Internal},
		{name: "内部错误", request: &blogopenv1.GetUserInfoRequest{UserId: 1}, err: errors.New("database secret details"), want: codes.Internal},
	}

	// 2. 逐项验证状态映射且内部错误详情不泄漏
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := &fakeUserQuery{err: test.err}
			service := NewOpenUserGRPCServer(query, fakeGRPCRegionResolver{})
			_, err := service.GetUserBasicInfo(context.Background(), test.request)
			if got := status.Code(err); got != test.want {
				t.Fatalf("status.Code() = %v, want %v; err = %v", got, test.want, err)
			}
			if test.want == codes.Internal && err.Error() == "database secret details" {
				t.Fatalf("internal error leaked dependency detail: %v", err)
			}
		})
	}
}
