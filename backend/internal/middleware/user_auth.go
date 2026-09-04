package middleware

import (
	"errors"
	"strings"

	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"codeup.aliyun.com/qimao/leo/lib/errassets"
	"codeup.aliyun.com/qimao/leo/lib/render"
	"github.com/gin-gonic/gin"
)

const (
	unauthenticatedCode  = 44030101 // unauthenticatedCode 表示请求没有有效登录身份。
	permissionDeniedCode = 44030102 // permissionDeniedCode 表示用户没有管理员权限。
	systemBusyCode       = 47010101 // systemBusyCode 表示依赖异常导致请求暂时不可用。
)

// UserAuthMiddleware 为认证、管理员和可选登录路由解析 Bearer Token 身份。
func UserAuthMiddleware(sessions userdomain.SessionRepository) gin.HandlerFunc {
	if sessions == nil {
		panic("用户认证中间件缺少会话仓储")
	}
	return func(ctx *gin.Context) {
		// 1. 公开路由不读取登录会话，可选登录路由允许游客继续
		path := ctx.Request.URL.Path
		required := requiresUserAuthentication(path)
		optional := strings.HasPrefix(path, "/optional/")
		if !required && !optional {
			ctx.Next()
			return
		}

		// 2. 校验 Bearer Header 并查询 Redis 会话
		token, ok := bearerToken(ctx.GetHeader("Authorization"))
		if !ok {
			if optional {
				ctx.Next()
				return
			}
			abortUnauthenticated(ctx, "未登录")
			return
		}
		session, err := sessions.FindByToken(ctx.Request.Context(), token)
		if err != nil {
			if errors.Is(err, userdomain.ErrSessionNotFound) {
				if optional {
					ctx.Next()
					return
				}
				abortUnauthenticated(ctx, "Token 无效")
				return
			}
			ctx.Negotiate(render.AbortWithError(ctx, errassets.NewError(systemBusyCode, "系统繁忙，请稍后再试")))
			return
		}

		// 3. 管理员路由必须由管理员角色访问
		if strings.HasPrefix(path, "/admin/") && session.Role != userdomain.RoleAdmin {
			ctx.Negotiate(render.AbortWithError(ctx, errassets.NewError(permissionDeniedCode, "无管理员权限")))
			return
		}

		// 4. 将稳定身份写入上下文供当前请求的应用服务使用
		identity.SetCurrentUser(ctx, identity.CurrentUser{ID: session.UserID, Role: session.Role})
		ctx.Next()
	}
}

// requiresUserAuthentication 判断路由是否要求用户登录。
func requiresUserAuthentication(path string) bool {
	// 1. 认证和管理员前缀统一要求登录
	return strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/admin/")
}

// bearerToken 解析标准 Bearer Token Header。
func bearerToken(authorization string) (string, bool) {
	// 1. Header 必须由 Bearer 类型和非空 Token 两部分组成
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// abortUnauthenticated 返回固定 HTTP 200 的未登录业务错误。
func abortUnauthenticated(ctx *gin.Context, message string) {
	// 1. 复用 Leo 错误渲染链，由统一响应中间件转换协议
	ctx.Negotiate(render.AbortWithError(ctx, errassets.NewError(unauthenticatedCode, message)))
}
