package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	createError   error                 // createError 是创建文章预设错误。
	createCalls   int                   // createCalls 是创建文章用例调用次数。
	updateCommand article.UpdateCommand // updateCommand 是收到的文章更新命令。
	updateError   error                 // updateError 是更新文章预设错误。
	publishID     uint64                // publishID 是收到的发布文章标识。
	publishAuthor uint64                // publishAuthor 是收到的发布作者标识。
	publishError  error                 // publishError 是发布文章预设错误。
	publicUserID  uint64                // publicUserID 是公开详情收到的可选用户标识。
	publicError   error                 // publicError 是公开详情预设错误。
	listCommand   article.ListCommand   // listCommand 是收到的后台列表命令。
	trashCommand  article.ListCommand   // trashCommand 是收到的垃圾箱列表命令。
	moveID        uint64                // moveID 是收到的软删除文章标识。
	recoverID     uint64                // recoverID 是收到的恢复文章标识。
	clearID       uint64                // clearID 是收到的彻底删除文章标识。
	actionAuthor  uint64                // actionAuthor 是文章管理操作的当前作者标识。
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

// Update 记录测试文章更新命令。
func (f *articleFake) Update(_ context.Context, command article.UpdateCommand) error {
	// 1. 保存协议层转换后的更新命令
	f.updateCommand = command
	return f.updateError
}

// Publish 记录测试文章发布参数。
func (f *articleFake) Publish(_ context.Context, articleID, authorID uint64) error {
	// 1. 保存协议层传递的文章和作者标识
	f.publishID = articleID
	f.publishAuthor = authorID
	return f.publishError
}

// PublicDetail 返回与可选登录身份对应的测试公开详情。
func (f *articleFake) PublicDetail(ctx context.Context, _ uint64, userID uint64) (*entity.Detail, error) {
	// 1. 记录可选登录身份并返回预设错误
	f.publicUserID = userID
	if f.publicError != nil {
		return nil, f.publicError
	}

	// 2. 登录用户返回已点赞，游客固定返回未点赞
	detail, err := f.Detail(ctx, 1, userID)
	if err != nil {
		return nil, err
	}
	detail.IsLiked = userID > 0
	return detail, nil
}

// List 记录后台文章列表命令。
func (f *articleFake) List(_ context.Context, command article.ListCommand) (*article.ListResult, error) {
	// 1. 保存协议层转换后的筛选和分页参数
	f.listCommand = command
	return testArticleListResult(), nil
}

// TrashList 记录垃圾箱文章列表命令。
func (f *articleFake) TrashList(_ context.Context, command article.ListCommand) (*article.ListResult, error) {
	// 1. 保存协议层转换后的垃圾箱分页参数
	f.trashCommand = command
	return testArticleListResult(), nil
}

// MoveToTrash 记录文章软删除参数。
func (f *articleFake) MoveToTrash(_ context.Context, articleID, authorID uint64) error {
	// 1. 保存待移入垃圾箱的文章和作者标识
	f.moveID = articleID
	f.actionAuthor = authorID
	return nil
}

// Recover 记录文章恢复参数。
func (f *articleFake) Recover(_ context.Context, articleID, authorID uint64) error {
	// 1. 保存待恢复的文章和作者标识
	f.recoverID = articleID
	f.actionAuthor = authorID
	return nil
}

// Clear 记录文章彻底删除参数。
func (f *articleFake) Clear(_ context.Context, articleID, authorID uint64) error {
	// 1. 保存待彻底删除的文章和作者标识
	f.clearID = articleID
	f.actionAuthor = authorID
	return nil
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

// StageDelete 模拟可提交或回滚的正文图片对象删除。
func (articleStorageFake) StageDelete(context.Context, string) (article.StagedObjectDeletion, error) {
	// 1. 应用层测试返回无副作用的暂存删除操作
	return articleStagedDeletionFake{}, nil
}

// ListStagedDeletions 返回空的启动恢复记录。
func (articleStorageFake) ListStagedDeletions(context.Context) ([]article.StagedObjectDeletion, error) {
	// 1. 应用层测试没有进程中断遗留对象
	return nil, nil
}

// articleStagedDeletionFake 模拟对象删除提交和回滚操作。
type articleStagedDeletionFake struct{}

// OriginalKey 返回空测试对象键。
func (articleStagedDeletionFake) OriginalKey() string {
	// 1. 应用层测试不依赖对象恢复决策
	return ""
}

// Commit 模拟完成对象删除。
func (articleStagedDeletionFake) Commit(context.Context) error {
	// 1. 应用层测试不执行真实对象清理
	return nil
}

// Rollback 模拟恢复对象。
func (articleStagedDeletionFake) Rollback(context.Context) error {
	// 1. 应用层测试不执行真实对象恢复
	return nil
}

// testArticleListResult 创建固定后台文章列表响应。
func testArticleListResult() *article.ListResult {
	// 1. 返回包含分页元数据的固定文章列表
	return &article.ListResult{
		Articles: []*entity.Article{{ID: 9, Title: "文章", Tags: []string{"Go"}, Status: article.StatusDraft, CreatedTime: time.Unix(10, 0), UpdatedTime: time.Unix(20, 0)}},
		LastID:   9, Total: 1, Page: 1, PageSize: 10,
	}
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

// TestArticleUpdateAndPublishHTTPContract 验证更新与发布接口的身份转换和空数据契约。
func TestArticleUpdateAndPublishHTTPContract(t *testing.T) {
	// 1. 定义更新和发布两个管理员操作
	tests := []struct {
		name        string // name 是接口场景。
		path        string // path 是请求路径。
		body        string // body 是 JSON 请求体。
		wantMessage string // wantMessage 是预期成功消息。
	}{
		{name: "更新文章", path: "/admin/article/update", body: `{"id":9,"title":"新标题","content":"正文","tags":["Go"],"status":2}`, wantMessage: "文章修改成功"},
		{name: "发布文章", path: "/admin/article/publish", body: `{"id":9}`, wantMessage: "文章发布成功"},
	}

	// 2. 逐项通过生成路由执行管理员请求
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := &articleFake{}
			router := gin.New()
			router.Use(middleware.UnifiedResponseMiddleware())
			router.Use(func(ctx *gin.Context) {
				identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 2})
				ctx.Next()
			})
			articleapi.RegisterArticleServiceHTTPServerController(router.Group(""), NewArticleServer(useCase, articleStorageFake{}, fakeRegionResolver{region: "浙江"}))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)

			// 2.1 校验成功消息、data=null 和作者身份传递
			var envelope struct {
				Success bool            `json:"success"` // Success 是业务成功标记。
				Message string          `json:"message"` // Message 是业务消息。
				Data    json.RawMessage `json:"data"`    // Data 是业务数据。
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if !envelope.Success || envelope.Message != test.wantMessage || string(envelope.Data) != "null" {
				t.Fatalf("response = %s", response.Body.String())
			}
			if test.path == "/admin/article/update" && (useCase.updateCommand.ArticleID != 9 || useCase.updateCommand.AuthorID != 7) {
				t.Fatalf("update command = %#v", useCase.updateCommand)
			}
			if test.path == "/admin/article/publish" && (useCase.publishID != 9 || useCase.publishAuthor != 7) {
				t.Fatalf("publish ID = %d, author ID = %d", useCase.publishID, useCase.publishAuthor)
			}
		})
	}
}

// TestPublicArticleDetailUsesOptionalIdentity 验证公开详情区分游客和登录用户点赞状态。
func TestPublicArticleDetailUsesOptionalIdentity(t *testing.T) {
	// 1. 定义游客和登录用户两个公开读取场景
	tests := []struct {
		name      string // name 是接口场景。
		userID    uint64 // userID 是可选登录用户标识。
		wantLiked bool   // wantLiked 是预期点赞状态。
	}{
		{name: "游客固定未点赞"},
		{name: "登录用户查询实际状态", userID: 7, wantLiked: true},
	}

	// 2. 逐项通过公开路由验证可选身份
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := &articleFake{}
			router := gin.New()
			router.Use(middleware.UnifiedResponseMiddleware())
			if test.userID > 0 {
				router.Use(func(ctx *gin.Context) {
					identity.SetCurrentUser(ctx, identity.CurrentUser{ID: test.userID, Role: 1})
					ctx.Next()
				})
			}
			articleapi.RegisterArticleServiceHTTPServerController(router.Group(""), NewArticleServer(useCase, articleStorageFake{}, fakeRegionResolver{region: "浙江"}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/optional/article/detail?id=1", nil))

			// 2.1 响应点赞状态和领域用例收到的用户标识必须一致
			wantLikedJSON := []byte(`"is_liked":false`)
			if test.wantLiked {
				wantLikedJSON = []byte(`"is_liked":true`)
			}
			if !bytes.Contains(response.Body.Bytes(), wantLikedJSON) || useCase.publicUserID != test.userID {
				t.Fatalf("response = %s, user ID = %d", response.Body.String(), useCase.publicUserID)
			}
		})
	}
}

// TestArticleMutationAndPublicErrorsHTTPContract 验证文章权限、删除和未发表错误码。
func TestArticleMutationAndPublicErrorsHTTPContract(t *testing.T) {
	// 1. 定义更新、发布和公开读取的可预期失败场景
	tests := []struct {
		name     string       // name 是接口场景。
		method   string       // method 是 HTTP 请求方法。
		path     string       // path 是请求路径。
		body     string       // body 是 JSON 请求体。
		useCase  *articleFake // useCase 是领域用例预设结果。
		wantCode int          // wantCode 是预期业务错误码。
	}{
		{name: "非作者不能更新", method: http.MethodPost, path: "/admin/article/update", body: `{"id":9,"title":"标题","content":"正文","status":2}`, useCase: &articleFake{updateError: article.ErrArticleNotOwned}, wantCode: codeArticleNotOwned},
		{name: "已删除文章不能发布", method: http.MethodPost, path: "/admin/article/publish", body: `{"id":9}`, useCase: &articleFake{publishError: article.ErrArticleDeleted}, wantCode: codeArticleDeleted},
		{name: "草稿不能公开读取", method: http.MethodGet, path: "/optional/article/detail?id=9", useCase: &articleFake{publicError: article.ErrArticleNotPublished}, wantCode: codeArticleNotPublished},
	}

	// 2. 逐项通过生成路由验证稳定业务码
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(middleware.UnifiedResponseMiddleware())
			router.Use(func(ctx *gin.Context) {
				if strings.HasPrefix(test.path, "/admin/") {
					identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 2})
				}
				ctx.Next()
			})
			articleapi.RegisterArticleServiceHTTPServerController(router.Group(""), NewArticleServer(test.useCase, articleStorageFake{}, fakeRegionResolver{region: "浙江"}))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)

			// 2.1 错误响应保持 HTTP 200 并返回预期业务码
			var envelope struct {
				Success bool `json:"success"` // Success 是业务成功标记。
				Code    int  `json:"code"`    // Code 是业务错误码。
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || envelope.Success || envelope.Code != test.wantCode {
				t.Fatalf("response = %s", response.Body.String())
			}
		})
	}
}

// TestArticleAdminListsHTTPContract 验证后台列表和垃圾箱列表的分页协议。
func TestArticleAdminListsHTTPContract(t *testing.T) {
	// 1. 定义普通后台列表和垃圾箱列表请求
	tests := []struct {
		name    string // name 是接口场景。
		path    string // path 是请求路径。
		isTrash bool   // isTrash 表示是否调用垃圾箱列表。
	}{
		{name: "后台文章列表", path: "/admin/article/list?status=-1&page=1&page_size=10&is_desc=true"},
		{name: "垃圾箱列表", path: "/admin/article/trash/list?status=1&page=1&page_size=10&is_desc=true", isTrash: true},
	}

	// 2. 逐项通过生成路由验证身份、筛选参数和响应字段
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := &articleFake{}
			router := gin.New()
			router.Use(middleware.UnifiedResponseMiddleware())
			router.Use(func(ctx *gin.Context) {
				identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 2})
				ctx.Next()
			})
			articleapi.RegisterArticleServiceHTTPServerController(router.Group(""), NewArticleServer(useCase, articleStorageFake{}, fakeRegionResolver{region: "浙江"}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))

			// 2.1 校验分页元数据和列表字段
			if !bytes.Contains(response.Body.Bytes(), []byte(`"last_id":9`)) ||
				!bytes.Contains(response.Body.Bytes(), []byte(`"total":1`)) ||
				!bytes.Contains(response.Body.Bytes(), []byte(`"title":"文章"`)) {
				t.Fatalf("response = %s", response.Body.String())
			}
			command := useCase.listCommand
			if test.isTrash {
				command = useCase.trashCommand
			}
			if command.AuthorID != 7 || command.Page != 1 || command.PageSize != 10 || !command.IsDesc {
				t.Fatalf("command = %#v", command)
			}
		})
	}
}

// TestArticleTrashMutationsHTTPContract 验证软删除、恢复和彻底删除的空数据契约。
func TestArticleTrashMutationsHTTPContract(t *testing.T) {
	// 1. 定义三种文章管理操作及兼容成功消息
	tests := []struct {
		name        string // name 是接口场景。
		path        string // path 是请求路径。
		wantMessage string // wantMessage 是预期成功消息。
	}{
		{name: "移入垃圾箱", path: "/admin/article/delete", wantMessage: "文章删除成功"},
		{name: "恢复为草稿", path: "/admin/article/trash/recover", wantMessage: "文章恢复成功"},
		{name: "彻底删除", path: "/admin/article/trash/clear", wantMessage: "文章彻底删除成功"},
	}

	// 2. 逐项通过生成路由执行当前作者操作
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := &articleFake{}
			router := gin.New()
			router.Use(middleware.UnifiedResponseMiddleware())
			router.Use(func(ctx *gin.Context) {
				identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 2})
				ctx.Next()
			})
			articleapi.RegisterArticleServiceHTTPServerController(router.Group(""), NewArticleServer(useCase, articleStorageFake{}, fakeRegionResolver{region: "浙江"}))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(`{"id":9,"status":2}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)

			// 2.1 校验固定 HTTP 200、data=null 和作者身份传递
			var envelope struct {
				Success bool            `json:"success"` // Success 是业务成功标记。
				Message string          `json:"message"` // Message 是业务消息。
				Data    json.RawMessage `json:"data"`    // Data 是业务数据。
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || !envelope.Success || envelope.Message != test.wantMessage || string(envelope.Data) != "null" || useCase.actionAuthor != 7 {
				t.Fatalf("response = %s, author ID = %d", response.Body.String(), useCase.actionAuthor)
			}
			if useCase.moveID != 9 && useCase.recoverID != 9 && useCase.clearID != 9 {
				t.Fatalf("mutation IDs = %d/%d/%d", useCase.moveID, useCase.recoverID, useCase.clearID)
			}
		})
	}
}
