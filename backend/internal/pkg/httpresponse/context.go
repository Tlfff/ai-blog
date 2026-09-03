// Package httpresponse 定义统一 HTTP 响应所需的上下文元数据。
package httpresponse

import "github.com/gin-gonic/gin"

const (
	successMessageKey = "http_response_success_message"
	nullDataKey       = "http_response_null_data"
)

// SetSuccess 设置当前请求成功时返回的业务消息。
func SetSuccess(ctx *gin.Context, message string, nullData bool) {
	// 1. 保存生成代码无法表达的成功响应元数据
	ctx.Set(successMessageKey, message)
	ctx.Set(nullDataKey, nullData)
}

// SuccessMetadata 读取成功响应的业务消息和空数据标记。
func SuccessMetadata(ctx *gin.Context) (string, bool) {
	// 1. 读取可选成功消息，缺失时使用通用文案
	message := "请求成功"
	if value, exists := ctx.Get(successMessageKey); exists {
		if configured, ok := value.(string); ok && configured != "" {
			message = configured
		}
	}

	// 2. 读取业务明确要求返回 null 的标记
	nullData, exists := ctx.Get(nullDataKey)
	if !exists {
		return message, false
	}
	shouldNull, ok := nullData.(bool)
	return message, ok && shouldNull
}
