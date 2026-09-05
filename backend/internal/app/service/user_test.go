package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	userapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/user"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/middleware"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"github.com/gin-gonic/gin"
)

// TestMain 在并行用例启动前一次性配置 Gin 测试模式。
func TestMain(testSuite *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(testSuite.Run())
}

// TestUserHTTPRegister 验证生成 HTTP 接口的成功、冲突和参数校验响应。
func TestUserHTTPRegister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string // name 是测试场景名称。
		body        string // body 是注册请求 JSON。
		registerErr error  // registerErr 是预设注册错误。
		wantSuccess bool   // wantSuccess 是预期业务成功标记。
		wantCode    int    // wantCode 是预期业务码。
		wantMessage string // wantMessage 是预期业务消息。
	}{
		{name: "注册成功", body: `{"nickname":"tester","phone":"13800138000","password":"secret1"}`, wantSuccess: true, wantMessage: "注册成功"},
		{name: "昵称冲突", body: `{"nickname":"tester","phone":"13800138000","password":"secret1"}`, registerErr: userdomain.ErrNicknameExists, wantCode: codeNicknameExists, wantMessage: "用户昵称已存在"},
		{name: "手机号冲突", body: `{"nickname":"tester","phone":"13800138000","password":"secret1"}`, registerErr: userdomain.ErrPhoneExists, wantCode: codePhoneExists, wantMessage: "手机号已存在"},
		{name: "参数校验失败", body: `{"nickname":"","phone":"13800138000","password":"short"}`, wantCode: 44010102},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			useCase := &fakeUserUseCase{registerErr: tt.registerErr}
			response := performUserRequest(useCase, http.MethodPost, "/user/register", tt.body, nil)
			if response.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200", response.Code)
			}
			envelope := decodeEnvelope(t, response)
			if envelope.Success != tt.wantSuccess || envelope.Code != tt.wantCode {
				t.Fatalf("response = %#v", envelope)
			}
			if tt.wantMessage != "" && envelope.Message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", envelope.Message, tt.wantMessage)
			}
			if tt.wantSuccess && string(envelope.Data) != "null" {
				t.Fatalf("data = %s, want null", envelope.Data)
			}
		})
	}
}

// TestUserHTTPProfileFailures 验证资料不存在和昵称冲突的稳定错误响应。
func TestUserHTTPProfileFailures(t *testing.T) {
	t.Parallel()

	// 1. 公开资料不存在时返回用户领域业务码
	missing := performUserRequest(
		&fakeUserUseCase{profileErr: userdomain.ErrUserNotFound},
		http.MethodGet,
		"/user/profile?user_id=7",
		"",
		nil,
	)
	missingEnvelope := decodeEnvelope(t, missing)
	if missingEnvelope.Success || missingEnvelope.Code != codeUserNotFound {
		t.Fatalf("missing profile response = %s", missing.Body.String())
	}

	// 2. 资料更新昵称冲突时返回昵称唯一性业务码
	withIdentity := middleware.UserAuthMiddleware(&fakeAppSessionRepository{
		session: &userdomain.Session{UserID: 7, Role: userdomain.RoleUser},
	})
	conflict := performUserRequest(
		&fakeUserUseCase{updateErr: userdomain.ErrNicknameExists},
		http.MethodPost,
		"/auth/my/profile/update",
		`{"nickname":"used","avatar":""}`,
		withIdentity,
	)
	conflictEnvelope := decodeEnvelope(t, conflict)
	if conflictEnvelope.Success || conflictEnvelope.Code != codeNicknameExists {
		t.Fatalf("update conflict response = %s", conflict.Body.String())
	}
}

// TestUserHTTPProfilesAndUpdate 验证公开资料过滤、完整资料和资料更新响应。
func TestUserHTTPProfilesAndUpdate(t *testing.T) {
	t.Parallel()

	loginTime := time.Unix(1_700_000_000, 0)
	useCase := &fakeUserUseCase{profile: &entity.User{
		ID:            7,
		Nickname:      "tester",
		Phone:         "13800138000",
		Password:      "secret",
		Avatar:        "avatar.png",
		Role:          userdomain.RoleAdmin,
		Status:        userdomain.StatusNormal,
		LastLoginTime: loginTime,
		LastLoginIP:   "北京市",
	}}

	publicResponse := performUserRequest(useCase, http.MethodGet, "/user/profile?user_id=7", "", nil)
	publicEnvelope := decodeEnvelope(t, publicResponse)
	if !publicEnvelope.Success || bytes.Contains(publicEnvelope.Data, []byte("phone")) || bytes.Contains(publicEnvelope.Data, []byte("role")) {
		t.Fatalf("public response = %s", publicResponse.Body.String())
	}

	withIdentity := middleware.UserAuthMiddleware(&fakeAppSessionRepository{
		session: &userdomain.Session{UserID: 7, Role: userdomain.RoleAdmin},
	})
	myResponse := performUserRequest(useCase, http.MethodGet, "/auth/my/profile", "", withIdentity)
	myEnvelope := decodeEnvelope(t, myResponse)
	if !myEnvelope.Success || !bytes.Contains(myEnvelope.Data, []byte(`"last_login_time":1700000000`)) {
		t.Fatalf("my profile response = %s", myResponse.Body.String())
	}

	updateResponse := performUserRequest(useCase, http.MethodPost, "/auth/my/profile/update", `{"nickname":"new","avatar":"new.png"}`, withIdentity)
	updateEnvelope := decodeEnvelope(t, updateResponse)
	if !updateEnvelope.Success || updateEnvelope.Message != "个人资料修改成功" || string(updateEnvelope.Data) != "null" {
		t.Fatalf("update response = %s", updateResponse.Body.String())
	}
	if useCase.updated.UserID != 7 || useCase.updated.Nickname != "new" || useCase.updated.Avatar != "new.png" {
		t.Fatalf("update command = %#v", useCase.updated)
	}
}

// TestUserHTTPRequiresCurrentUser 验证当前用户资料接口拒绝缺少身份的请求。
func TestUserHTTPRequiresCurrentUser(t *testing.T) {
	t.Parallel()

	response := performUserRequest(&fakeUserUseCase{}, http.MethodGet, "/auth/my/profile", "", nil)
	envelope := decodeEnvelope(t, response)
	if envelope.Success || envelope.Code != codeUnauthenticated {
		t.Fatalf("response = %s", response.Body.String())
	}
}

type responseEnvelope struct {
	Success bool            `json:"success"` // Success 是业务成功标记。
	Code    int             `json:"code"`    // Code 是业务码。
	Message string          `json:"message"` // Message 是业务消息。
	Data    json.RawMessage `json:"data"`    // Data 是业务响应数据。
}

// performUserRequest 通过生成路由执行一次用户 HTTP 请求。
//
// 参数说明：
//   - useCase：用户上下文业务能力替身。
//   - method：HTTP 请求方法。
//   - path：包含 Query 的请求路径。
//   - body：JSON 请求体，可以为空。
//   - before：在生成路由前执行的可选中间件。
func performUserRequest(useCase userdomain.UseCase, method, path, body string, before gin.HandlerFunc) *httptest.ResponseRecorder {
	router := gin.New()
	router.Use(middleware.UnifiedResponseMiddleware())
	if before != nil {
		router.Use(before)
	}
	userapi.RegisterUserServiceHTTPServerController(router.Group(""), &UserService{useCase: useCase, regionResolver: fakeRegionResolver{region: "测试地区"}})
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if before != nil {
		request.Header.Set("Authorization", "Bearer valid")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

// decodeEnvelope 解析统一 HTTP 响应。
func decodeEnvelope(t *testing.T, response *httptest.ResponseRecorder) responseEnvelope {
	t.Helper()
	var envelope responseEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
	return envelope
}

type fakeUserUseCase struct {
	registerErr error                           // registerErr 是预设注册错误。
	profile     *entity.User                    // profile 是预设用户资料。
	profileErr  error                           // profileErr 是预设资料查询错误。
	updated     userdomain.UpdateProfileCommand // updated 记录资料更新命令。
	updateErr   error                           // updateErr 是预设资料更新错误。
}

type fakeAppSessionRepository struct {
	session *userdomain.Session // session 是预设登录会话。
}

// FindByToken 返回应用服务测试使用的登录会话。
func (f *fakeAppSessionRepository) FindByToken(context.Context, string) (*userdomain.Session, error) {
	// 1. 返回预设会话供认证中间件注入身份
	return f.session, nil
}

// Register 返回测试预设的注册结果。
func (f *fakeUserUseCase) Register(context.Context, userdomain.RegisterCommand) error {
	return f.registerErr
}

// GetProfile 返回测试预设的用户资料。
func (f *fakeUserUseCase) GetProfile(context.Context, uint64) (*entity.User, error) {
	if f.profileErr != nil {
		return nil, f.profileErr
	}
	if f.profile == nil {
		return nil, errors.New("profile not configured")
	}
	clone := *f.profile
	return &clone, nil
}

// UpdateProfile 记录资料更新命令并返回预设结果。
func (f *fakeUserUseCase) UpdateProfile(_ context.Context, command userdomain.UpdateProfileCommand) error {
	f.updated = command
	return f.updateErr
}

// VerifyOldPassword 返回固定测试改密凭证。
func (f *fakeUserUseCase) VerifyOldPassword(context.Context, uint64, string) (string, error) {
	// 1. 返回固定凭证供 HTTP 协议测试
	return "change-token", nil
}

// ChangePassword 模拟成功修改密码。
func (f *fakeUserUseCase) ChangePassword(context.Context, userdomain.ChangePasswordCommand) error {
	// 1. HTTP 测试默认领域改密成功
	return nil
}

// UpdatePhone 模拟成功修改手机号。
func (f *fakeUserUseCase) UpdatePhone(context.Context, userdomain.UpdatePhoneCommand) error {
	// 1. HTTP 测试默认领域手机号更新成功
	return nil
}

// GetAvatarUploadURL 返回固定头像直传凭证。
func (f *fakeUserUseCase) GetAvatarUploadURL(context.Context, uint64, string) (*userdomain.AvatarUploadResult, error) {
	// 1. 返回固定上传地址和当前用户对象 Key
	return &userdomain.AvatarUploadResult{UploadURL: "signed", ObjectKey: "avatar/7/a.png"}, nil
}

// ConfirmAvatar 返回固定头像公开地址。
func (f *fakeUserUseCase) ConfirmAvatar(context.Context, uint64, string) (string, error) {
	// 1. 返回固定公开地址供响应转换测试
	return "https://public/avatar/7/a.png", nil
}

// TestUserHTTPAuthRoutes 验证登录与退出由 Proto 生成路由处理，并严格校验 Bearer Token。
func TestUserHTTPAuthRoutes(t *testing.T) {
	auth := &fakeAuthUseCase{result: &userdomain.LoginResult{AccessToken: "token"}}
	server := &UserService{authUseCase: auth, trustedProxyCIDRs: []string{"10.0.0.0/8"}}
	router := gin.New()
	router.Use(middleware.UnifiedResponseMiddleware())
	userapi.RegisterUserServiceHTTPServerController(router.Group(""), server)

	login := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/user/login", bytes.NewBufferString(`{"nickname":"tester","password":"secret1"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	request.RemoteAddr = "10.0.0.8:1234"
	router.ServeHTTP(login, request)
	if envelope := decodeEnvelope(t, login); !envelope.Success || !bytes.Contains(envelope.Data, []byte(`"access_token":"token"`)) {
		t.Fatalf("login response = %s", login.Body.String())
	}
	if auth.command.Nickname != "tester" || auth.command.ClientIP != "203.0.113.9" {
		t.Fatalf("login command = %#v", auth.command)
	}

	logout := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/auth/my/logout", nil)
	request.Header.Set("Authorization", "Basic token")
	router.ServeHTTP(logout, request)
	if envelope := decodeEnvelope(t, logout); envelope.Success || envelope.Code != codeUnauthenticated {
		t.Fatalf("logout response = %s", logout.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/auth/my/logout", nil)
	request.Header.Set("Authorization", "Bearer token")
	logout = httptest.NewRecorder()
	router.ServeHTTP(logout, request)
	if envelope := decodeEnvelope(t, logout); !envelope.Success || auth.logoutToken != "token" {
		t.Fatalf("logout response = %s token=%q", logout.Body.String(), auth.logoutToken)
	}
}

// TestUserProfileRegionDoesNotLeakRawIP 验证资料响应只返回地区文案，不直接暴露原始 IP。
func TestUserProfileRegionDoesNotLeakRawIP(t *testing.T) {
	useCase := &fakeUserUseCase{profile: &entity.User{ID: 7, Nickname: "tester", LastLoginIP: "203.0.113.8"}}
	server := &UserService{useCase: useCase, regionResolver: fakeRegionResolver{region: "广东"}}
	router := gin.New()
	router.Use(middleware.UnifiedResponseMiddleware())
	router.Use(func(ctx *gin.Context) { identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7}); ctx.Next() })
	userapi.RegisterUserServiceHTTPServerController(router.Group(""), server)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/auth/my/profile", nil))
	if bytes.Contains(response.Body.Bytes(), []byte("203.0.113.8")) || !bytes.Contains(response.Body.Bytes(), []byte(`"last_login_ip":"广东"`)) {
		t.Fatalf("profile response = %s", response.Body.String())
	}
}

type fakeAuthUseCase struct {
	result      *userdomain.LoginResult // result 是预设登录结果。
	command     userdomain.LoginCommand // command 记录登录命令。
	logoutToken string                  // logoutToken 记录退出 Token。
}

// Login 记录登录命令并返回预设结果。
func (f *fakeAuthUseCase) Login(_ context.Context, command userdomain.LoginCommand) (*userdomain.LoginResult, error) {
	// 1. 记录领域命令并返回预设登录结果
	f.command = command
	return f.result, nil
}

// Logout 记录当前设备退出 Token。
func (f *fakeAuthUseCase) Logout(_ context.Context, token string) error {
	// 1. 记录当前设备退出使用的 Token
	f.logoutToken = token
	return nil
}

type fakeRegionResolver struct {
	region string // region 是预设地区文案。
}

// Resolve 返回测试预设的脱敏地区文案。
func (f fakeRegionResolver) Resolve(string) string {
	// 1. 返回预设地区文案，确保测试不会暴露原始 IP
	return f.region
}
