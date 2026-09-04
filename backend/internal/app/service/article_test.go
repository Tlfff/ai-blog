package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	articleapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/article"
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/middleware"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"github.com/gin-gonic/gin"
)

// articleFake 模拟文章应用服务依赖的领域用例。
type articleFake struct {
	createError error // createError 是创建文章预设错误。
	createCalls int   // createCalls 是创建文章用例调用次数。
}

// UploadImage 返回测试图片上传凭证。
func (*articleFake) UploadImage(context.Context, uint64, string) (*article.UploadResult, error) {
	// 1. 返回固定上传凭证
	return &article.UploadResult{ImageID: 8, UploadURL: "upload", URL: "preview"}, nil
}

// Create 返回预设文章创建结果。
func (f *articleFake) Create(context.Context, article.CreateCommand) error {
	// 1. 记录调用并返回领域层已完成防重后的结果
	f.createCalls++
	return f.createError
}

// Detail 返回带点赞状态和图片映射的测试详情。
func (*articleFake) Detail(context.Context, uint64, uint64) (*entity.Detail, error) {
	// 1. 返回固定后台详情
	return &entity.Detail{
		Article: &entity.Article{ID: 1, Title: "标题", Content: "正文", Status: 3, LikeCount: 2,
			CreatedTime: time.Unix(10, 0), UpdatedTime: time.Unix(20, 0)},
		AuthorNickname: "作者", AuthorIP: "203.0.113.8", IsLiked: true, Images: []*entity.Image{{ID: 8, ObjectKey: "image.png"}},
	}, nil
}

// articleStorageFake 返回固定公开图片地址。
type articleStorageFake struct{}

// PresignPut 返回固定测试上传地址。
func (articleStorageFake) PresignPut(context.Context, string, time.Duration) (string, error) {
	// 1. 返回固定地址
	return "upload", nil
}

// PublicURL 返回固定测试公开地址。
func (articleStorageFake) PublicURL(string) string {
	// 1. 返回固定地址
	return "preview"
}

// TestArticleCreateHTTPContract 验证创建成功和重复提交的完整 HTTP 契约。
func TestArticleCreateHTTPContract(t *testing.T) {
	// 1. 定义创建成功和可预期领域错误的响应场景
	tests := []struct {
		name        string // name 是接口场景。
		createError error  // createError 是领域层创建结果。
		wantSuccess bool   // wantSuccess 是预期成功标记。
		wantCode    int    // wantCode 是预期业务码。
		wantMessage string // wantMessage 是预期消息。
	}{
		{name: "创建成功返回空数据", wantSuccess: true, wantMessage: "文章创建成功"},
		{name: "两秒内重复提交", createError: article.ErrDuplicateSubmission, wantCode: codeArticleDuplicate, wantMessage: "请勿重复提交"},
		{name: "领域拒绝非法状态", createError: article.ErrInvalidStatus, wantCode: codeArticleInvalidStatus, wantMessage: "文章状态不合法"},
	}
	// 2. 逐项通过生成路由验证统一 HTTP 契约
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 1. 通过生成路由执行管理员创建请求
			router := gin.New()
			router.Use(middleware.UnifiedResponseMiddleware())
			router.Use(func(ctx *gin.Context) {
				identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 2})
				ctx.Next()
			})
			articleapi.RegisterArticleServiceHTTPServerController(router.Group(""), NewArticleServer(&articleFake{createError: test.createError}, articleStorageFake{}, fakeRegionResolver{region: "浙江"}))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/admin/article/create", bytes.NewBufferString(`{"title":"标题","content":"正文","status":2}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)

			// 2. 校验固定 HTTP 200 和完整统一响应结构
			var envelope struct {
				Success bool            `json:"success"` // Success 是业务成功标记。
				Code    int             `json:"code"`    // Code 是业务码。
				Message string          `json:"message"` // Message 是业务消息。
				Data    json.RawMessage `json:"data"`    // Data 是业务数据。
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || envelope.Success != test.wantSuccess || envelope.Code != test.wantCode || envelope.Message != test.wantMessage {
				t.Fatalf("response = %s", response.Body.String())
			}
			if string(envelope.Data) != "null" {
				t.Fatalf("data = %s, want null", envelope.Data)
			}
		})
	}
}

// TestArticleCreateRejectsOutOfRangeStatus 验证超出 int8 范围的状态不会绕过领域校验。
func TestArticleCreateRejectsOutOfRangeStatus(t *testing.T) {
	// 1. 通过生成路由提交会在 int8 转换后变成草稿的非法状态 258
	useCase := &articleFake{}
	router := gin.New()
	router.Use(middleware.UnifiedResponseMiddleware())
	router.Use(func(ctx *gin.Context) {
		identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 2})
		ctx.Next()
	})
	articleapi.RegisterArticleServiceHTTPServerController(router.Group(""), NewArticleServer(useCase, articleStorageFake{}, fakeRegionResolver{region: "浙江"}))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/article/create", bytes.NewBufferString(`{"title":"标题","content":"正文","status":258}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	// 2. Proto 校验应返回请求错误，且不得调用文章领域用例
	var envelope struct {
		Success bool   `json:"success"` // Success 是业务成功标记。
		Code    int    `json:"code"`    // Code 是业务码。
		Message string `json:"message"` // Message 是业务消息。
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || envelope.Success || envelope.Code != 44010102 || useCase.createCalls != 0 {
		t.Fatalf("response = %s, create calls = %d", response.Body.String(), useCase.createCalls)
	}
}

// TestArticleDetailHTTPIncludesLikeState 验证后台详情返回点赞状态和图片映射。
func TestArticleDetailHTTPIncludesLikeState(t *testing.T) {
	// 1. 通过生成详情路由执行管理员请求
	router := gin.New()
	router.Use(middleware.UnifiedResponseMiddleware())
	router.Use(func(ctx *gin.Context) {
		identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 2})
		ctx.Next()
	})
	articleapi.RegisterArticleServiceHTTPServerController(router.Group(""), NewArticleServer(&articleFake{}, articleStorageFake{}, fakeRegionResolver{region: "浙江"}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/article/me/detail?id=1", nil))

	// 2. 点赞状态、点赞数和图片映射必须进入响应
	if !bytes.Contains(response.Body.Bytes(), []byte(`"is_liked":true`)) ||
		!bytes.Contains(response.Body.Bytes(), []byte(`"like_count":2`)) ||
		!bytes.Contains(response.Body.Bytes(), []byte(`"url":"preview"`)) ||
		!bytes.Contains(response.Body.Bytes(), []byte(`"ip":"浙江"`)) {
		t.Fatalf("response = %s", response.Body.String())
	}
}
