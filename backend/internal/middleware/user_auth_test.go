package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"github.com/gin-gonic/gin"
)

// TestUserAuthMiddleware 验证公开路由、缺失 Token 和有效会话的身份行为。
func TestUserAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string              // name 是测试场景名称。
		path          string              // path 是请求路径。
		authorization string              // authorization 是认证 Header。
		session       *userdomain.Session // session 是预设登录会话。
		sessionErr    error               // sessionErr 是预设会话查询错误。
		wantStatus    int                 // wantStatus 是下游处理器写出的状态。
		wantLookup    bool                // wantLookup 表示是否应查询会话。
	}{
		{name: "公开路由跳过会话", path: "/user/profile", wantStatus: http.StatusNoContent},
		{name: "可选登录允许游客", path: "/optional/article/detail", wantStatus: http.StatusNoContent},
		{name: "可选登录忽略无效 Token", path: "/optional/article/detail", authorization: "Bearer missing", wantStatus: http.StatusNoContent, wantLookup: true},
		{name: "可选登录注入有效身份", path: "/optional/article/detail", authorization: "Bearer valid", session: &userdomain.Session{UserID: 7, Role: userdomain.RoleUser}, wantStatus: http.StatusNoContent, wantLookup: true},
		{name: "受保护路由缺少 Token", path: "/auth/my/profile", wantStatus: http.StatusOK},
		{name: "Bearer 格式错误", path: "/auth/my/profile", authorization: "Basic token", wantStatus: http.StatusOK},
		{name: "Token 不存在", path: "/auth/my/profile", authorization: "Bearer missing", wantStatus: http.StatusOK, wantLookup: true},
		{name: "Token 已过期", path: "/auth/my/profile", authorization: "Bearer expired", sessionErr: userdomain.ErrSessionNotFound, wantStatus: http.StatusOK, wantLookup: true},
		{name: "有效 Token 注入身份", path: "/auth/my/profile", authorization: "Bearer valid", session: &userdomain.Session{UserID: 7, Role: userdomain.RoleUser}, wantStatus: http.StatusNoContent, wantLookup: true},
		{name: "普通用户不能访问管理员路由", path: "/admin/article/list", authorization: "Bearer valid", session: &userdomain.Session{UserID: 7, Role: userdomain.RoleUser}, wantStatus: http.StatusOK, wantLookup: true},
		{name: "管理员可以访问管理员路由", path: "/admin/article/list", authorization: "Bearer valid", session: &userdomain.Session{UserID: 7, Role: userdomain.RoleAdmin}, wantStatus: http.StatusNoContent, wantLookup: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. 注册认证中间件和用于观察身份的下游处理器
			sessions := &fakeSessionRepository{session: tt.session, err: tt.sessionErr}
			router := gin.New()
			router.Use(UserAuthMiddleware(sessions))
			router.GET(tt.path, func(ctx *gin.Context) {
				if tt.session != nil {
					currentUser, ok := identity.FromContext(ctx)
					if !ok || currentUser.ID != tt.session.UserID {
						t.Fatalf("current user = %#v, exists %v", currentUser, ok)
					}
				}
				ctx.Status(http.StatusNoContent)
			})

			// 2. 执行请求并验证是否按路由和 Token 查询会话
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.Header.Set("Authorization", tt.authorization)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.wantStatus || sessions.called != tt.wantLookup {
				t.Fatalf("status = %d lookup = %v, want %d/%v", response.Code, sessions.called, tt.wantStatus, tt.wantLookup)
			}
		})
	}
}

type fakeSessionRepository struct {
	session *userdomain.Session // session 是预设登录会话。
	called  bool                // called 记录是否查询会话。
	err     error               // err 是预设会话查询错误。
}

// FindByToken 返回测试预设的登录会话。
func (f *fakeSessionRepository) FindByToken(context.Context, string) (*userdomain.Session, error) {
	// 1. 记录查询并返回预设会话或会话不存在错误
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	if f.session == nil {
		return nil, userdomain.ErrSessionNotFound
	}
	return f.session, nil
}
