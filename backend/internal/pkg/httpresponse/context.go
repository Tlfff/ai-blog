// Package httpresponse 定义统一 HTTP 响应所需的上下文元数据。
package httpresponse

import "github.com/gin-gonic/gin"

const (
	successMessageKey = "http_response_success_message"
	nullDataKey       = "http_response_null_data"
	dataOverrideKey   = "http_response_data_override"
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

// SetDataOverride 设置统一响应使用的原始 JSON data。
func SetDataOverride(ctx *gin.Context, data []byte) {
	// 1. 复制数据避免调用方后续修改底层字节
	ctx.Set(dataOverrideKey, append([]byte(nil), data...))
}

// DataOverride 读取统一响应的原始 JSON data。
func DataOverride(ctx *gin.Context) ([]byte, bool) {
	// 1. 仅接受非空字节切片覆盖生成代码响应
	value, exists := ctx.Get(dataOverrideKey)
	data, ok := value.([]byte)
	return data, exists && ok && len(data) > 0
}
