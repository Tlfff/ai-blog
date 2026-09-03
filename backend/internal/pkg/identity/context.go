// Package identity 定义应用入口传递当前调用方身份的稳定上下文接缝。
package identity

import "github.com/gin-gonic/gin"

const currentUserKey = "current_user"

// CurrentUser 是经过认证的当前用户身份。
type CurrentUser struct {
	ID   uint64 // ID 是当前用户标识。
	Role int8   // Role 是当前用户角色。
}

// SetCurrentUser 将认证结果写入 Gin 上下文。
func SetCurrentUser(ctx *gin.Context, currentUser CurrentUser) {
	// 1. 使用私有 Key 保存已认证身份
	ctx.Set(currentUserKey, currentUser)
}

// FromContext 读取当前用户身份。
func FromContext(ctx *gin.Context) (CurrentUser, bool) {
	// 1. 读取并验证认证中间件写入的身份类型
	value, exists := ctx.Get(currentUserKey)
	if !exists {
		return CurrentUser{}, false
	}
	currentUser, ok := value.(CurrentUser)
	return currentUser, ok && currentUser.ID > 0
}
