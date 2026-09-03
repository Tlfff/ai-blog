package middleware

import (
	"bytes"
	"encoding/json"
	"strconv"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"codeup.aliyun.com/qimao/leo/leo/log"
	"github.com/gin-gonic/gin"
)

// unifiedResponseWriter 缓存生成代码写出的响应体，以便统一包装协议。
type unifiedResponseWriter struct {
	gin.ResponseWriter              // ResponseWriter 是最终写入客户端的原始响应器。
	body               bytes.Buffer // body 缓存生成代码写出的 JSON。
}

// Write 缓存响应体，最终由中间件一次性写入客户端。
func (w *unifiedResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

// WriteString 缓存字符串响应体。
func (w *unifiedResponseWriter) WriteString(data string) (int, error) {
	return w.body.WriteString(data)
}

// generatedResponse 是 Proto HTTP 生成代码的响应结构。
type generatedResponse struct {
	Data   json.RawMessage `json:"data"`   // Data 是成功响应数据。
	Errors *generatedError `json:"errors"` // Errors 是失败响应信息。
}

// generatedError 是 Leo 错误渲染器的响应结构。
type generatedError struct {
	Code  json.RawMessage `json:"code"`  // Code 是字符串或数字格式的业务码。
	Title string          `json:"title"` // Title 是面向用户的错误消息。
}

// unifiedResponse 是博客 HTTP 接口的统一响应结构。
type unifiedResponse struct {
	Success bool            `json:"success"` // Success 表示业务是否成功。
	Code    int             `json:"code"`    // Code 是稳定业务码，成功为 0。
	Message string          `json:"message"` // Message 是业务结果说明。
	Data    json.RawMessage `json:"data"`    // Data 是业务数据，允许为 null。
}

// UnifiedResponseMiddleware 将生成代码的响应转换为博客统一协议。
func UnifiedResponseMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 1. 缓存生成代码或错误渲染器写出的 JSON
		writer := &unifiedResponseWriter{ResponseWriter: ctx.Writer}
		ctx.Writer = writer
		ctx.Next()
		ctx.Writer = writer.ResponseWriter

		// 2. 仅处理生成代码可识别的成功或错误响应
		var generated generatedResponse
		if err := json.Unmarshal(writer.body.Bytes(), &generated); err != nil ||
			(generated.Data == nil && generated.Errors == nil) {
			if _, writeErr := ctx.Writer.Write(writer.body.Bytes()); writeErr != nil {
				log.L().WithContext(ctx.Request.Context()).Error("写入原始 HTTP 响应失败", writeErr)
			}
			return
		}

		// 3. 写出固定 HTTP 200 的统一业务协议
		response := unifiedResponse{Data: json.RawMessage("null")}
		if generated.Errors != nil {
			response.Message = generated.Errors.Title
			response.Code = parseBusinessCode(generated.Errors.Code)
		} else {
			response.Success = true
			message, nullData := httpresponse.SuccessMetadata(ctx)
			response.Message = message
			if !nullData {
				response.Data = generated.Data
			}
		}
		payload, err := json.Marshal(response)
		if err != nil {
			log.L().WithContext(ctx.Request.Context()).Error("编码统一 HTTP 响应失败", err)
			if _, writeErr := ctx.Writer.Write(writer.body.Bytes()); writeErr != nil {
				log.L().WithContext(ctx.Request.Context()).Error("回退写入原始 HTTP 响应失败", writeErr)
			}
			return
		}
		ctx.Header("Content-Type", "application/json; charset=utf-8")
		ctx.Writer.Header().Del("Content-Length")
		ctx.Status(200)
		if _, err := ctx.Writer.Write(payload); err != nil {
			log.L().WithContext(ctx.Request.Context()).Error("写入统一 HTTP 响应失败", err)
		}
	}
}

// parseBusinessCode 兼容 Leo 错误码的字符串和数字 JSON 表示。
func parseBusinessCode(raw json.RawMessage) int {
	var code int
	if err := json.Unmarshal(raw, &code); err == nil {
		return code
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0
	}
	code, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return code
}
