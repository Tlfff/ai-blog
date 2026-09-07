package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"github.com/gin-gonic/gin"
)

// TestUnifiedResponseMiddlewareUsesDataOverride 验证标量接口可覆盖生成代码的对象 data。
func TestUnifiedResponseMiddlewareUsesDataOverride(t *testing.T) {
	// 1. 模拟生成代码先写对象，应用服务同时声明标量覆盖值
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(UnifiedResponseMiddleware())
	engine.GET("/count", func(ctx *gin.Context) {
		httpresponse.SetDataOverride(ctx, []byte("4"))
		_, _ = ctx.Writer.Write([]byte(`{"data":{"count":4}}`))
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/count", nil))

	// 2. 统一响应的 data 必须保持功能文档约定的数字
	want := `{"success":true,"code":0,"message":"请求成功","data":4}`
	if response.Body.String() != want {
		t.Fatalf("body=%s want=%s", response.Body.String(), want)
	}
}
