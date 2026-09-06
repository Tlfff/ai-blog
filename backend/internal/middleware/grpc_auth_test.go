package middleware

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	leolog "codeup.aliyun.com/qimao/leo/leo/log"
	leoslog "codeup.aliyun.com/qimao/leo/leo/log/slog"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	testJWTSecret  = "jwt-secret-that-is-at-least-32-bytes-long"
	testHMACSecret = "hmac-secret-that-is-at-least-32-bytes-long"
	testIssuer     = "blog-internal"
	testAccessKey  = "partner-a"
)

// memoryNonceStore 提供可并发测试的内存 Nonce 原子占用替身。
type memoryNonceStore struct {
	mu       sync.Mutex          // mu 保护并发测试中的占用状态。
	reserved map[string]struct{} // reserved 保存已占用的合作方与 Nonce 组合。
	err      error               // err 是测试注入的存储故障。
	lastTTL  time.Duration       // lastTTL 记录最近一次占用使用的有效期。
}

// ReserveGRPCNonce 原子记录合作方 Nonce，并在重复时返回 false。
func (s *memoryNonceStore) ReserveGRPCNonce(_ context.Context, accessKeyID, nonce string, ttl time.Duration) (bool, error) {
	// 1. 串行保护故障注入和重复占用判断
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return false, s.err
	}
	if s.reserved == nil {
		s.reserved = make(map[string]struct{})
	}
	key := accessKeyID + "\x00" + nonce
	if _, exists := s.reserved[key]; exists {
		return false, nil
	}
	s.reserved[key] = struct{}{}
	s.lastTTL = ttl
	return true, nil
}

// testAuthenticator 创建使用固定配置和时钟的认证器。
func testAuthenticator(t *testing.T, store userdomain.GRPCNonceStore, now time.Time) *GRPCAuthenticator {
	// 1. 使用满足强度约束的测试密钥构造认证器
	t.Helper()
	authenticator, err := NewGRPCAuthenticator(GRPCAuthSettings{
		JWTIssuer:     testIssuer,
		JWTSecret:     []byte(testJWTSecret),
		HMACWindow:    time.Minute,
		NonceTTL:      time.Minute,
		HMACAccessKey: map[string][]byte{testAccessKey: []byte(testHMACSecret)},
	}, store)
	if err != nil {
		t.Fatalf("NewGRPCAuthenticator() error = %v", err)
	}

	// 2. 固定时钟以稳定验证过期和边界行为
	authenticator.now = func() time.Time { return now }
	return authenticator
}

// signJWT 使用指定算法和声明生成测试 Token。
//
// 参数说明：
//   - t：当前测试上下文。
//   - secret：签名密钥。
//   - issuer：JWT 签发者。
//   - subject：内部调用方标识。
//   - issuedAt：Token 签发时间。
//   - expiresAt：Token 过期时间。
//   - method：待测试的 JWT 签名算法。
func signJWT(t *testing.T, secret, issuer, subject string, issuedAt, expiresAt time.Time, method jwt.SigningMethod) string {
	// 1. 组装注册声明并使用指定算法签名
	t.Helper()
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	token := jwt.NewWithClaims(method, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}

// invokeAuth 执行 Unary Interceptor 并读取注入的调用方身份。
//
// 参数说明：
//   - t：当前测试上下文。
//   - authenticator：待测试认证器。
//   - ctx：携带认证 Metadata 的请求上下文。
//   - request：待认证请求。
//   - method：完整 gRPC 方法名。
func invokeAuth(t *testing.T, authenticator *GRPCAuthenticator, ctx context.Context, request any, method string) (GRPCCaller, error) {
	// 1. 通过真实拦截器执行认证并记录 Handler 上下文身份
	t.Helper()
	var caller GRPCCaller
	_, err := authenticator.UnaryServerInterceptor()(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(handlerCtx context.Context, _ any) (any, error) {
		var ok bool
		caller, ok = GRPCCallerFromContext(handlerCtx)
		if !ok {
			t.Fatal("handler context missing GRPCCaller")
		}
		return request, nil
	})
	return caller, err
}

// TestGRPCJWTAuthentication 验证 HS256、签发者、签名、过期和必要声明。
func TestGRPCJWTAuthentication(t *testing.T) {
	// 1. 构造固定时钟下的成功和失败 JWT 场景
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string                  // name 是测试场景名称。
		token  func(*testing.T) string // token 创建当前场景的 JWT。
		want   codes.Code              // want 是期望的标准 gRPC Code。
		wantID string                  // wantID 是成功时注入的调用方标识。
		method string                  // method 是待访问的完整 RPC 方法。
	}{
		{
			name: "有效 HS256 JWT",
			token: func(t *testing.T) string {
				return signJWT(t, testJWTSecret, testIssuer, "article-service", now.Add(-time.Minute), now.Add(time.Minute), jwt.SigningMethodHS256)
			},
			want: codes.OK, wantID: "article-service", method: userBasicInfoMethod,
		},
		{
			name: "错误签发者",
			token: func(t *testing.T) string {
				return signJWT(t, testJWTSecret, "wrong-issuer", "article-service", now.Add(-time.Minute), now.Add(time.Minute), jwt.SigningMethodHS256)
			},
			want: codes.Unauthenticated, method: userBasicInfoMethod,
		},
		{
			name: "错误签名",
			token: func(t *testing.T) string {
				return signJWT(t, "another-secret-that-is-at-least-32-bytes", testIssuer, "article-service", now.Add(-time.Minute), now.Add(time.Minute), jwt.SigningMethodHS256)
			},
			want: codes.Unauthenticated, method: userBasicInfoMethod,
		},
		{
			name: "过期",
			token: func(t *testing.T) string {
				return signJWT(t, testJWTSecret, testIssuer, "article-service", now.Add(-2*time.Minute), now.Add(-time.Second), jwt.SigningMethodHS256)
			},
			want: codes.Unauthenticated, method: userBasicInfoMethod,
		},
		{
			name: "缺少 issued at",
			token: func(t *testing.T) string {
				claims := jwt.RegisteredClaims{Issuer: testIssuer, Subject: "article-service", ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				signed, err := token.SignedString([]byte(testJWTSecret))
				if err != nil {
					t.Fatal(err)
				}
				return signed
			},
			want: codes.Unauthenticated, method: userBasicInfoMethod,
		},
		{
			name: "缺少 subject",
			token: func(t *testing.T) string {
				return signJWT(t, testJWTSecret, testIssuer, "", now.Add(-time.Minute), now.Add(time.Minute), jwt.SigningMethodHS256)
			},
			want: codes.Unauthenticated, method: userBasicInfoMethod,
		},
		{
			name: "拒绝非 HS256",
			token: func(t *testing.T) string {
				return signJWT(t, testJWTSecret, testIssuer, "article-service", now.Add(-time.Minute), now.Add(time.Minute), jwt.SigningMethodHS384)
			},
			want: codes.Unauthenticated, method: userBasicInfoMethod,
		},
	}
	// 2. 逐项验证状态码及成功身份注入
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator := testAuthenticator(t, &memoryNonceStore{}, now)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(metadataAuthorization, "Bearer "+test.token(t)))
			caller, err := invokeAuth(t, authenticator, ctx, &blogopenv1.GetUserInfoRequest{UserId: 42}, test.method)
			if got := status.Code(err); got != test.want {
				t.Fatalf("status.Code() = %v, want %v; err = %v", got, test.want, err)
			}
			if test.want == codes.OK && (caller.Kind != GRPCCallerInternal || caller.ID != test.wantID) {
				t.Fatalf("caller = %#v", caller)
			}
		})
	}
}

// TestGRPCHMACAuthenticationAndReplayProtection 验证有效签名及重复 Nonce 失败。
func TestGRPCHMACAuthenticationAndReplayProtection(t *testing.T) {
	// 1. 生成有效签名并验证首次请求身份与 Nonce TTL
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	store := &memoryNonceStore{}
	authenticator := testAuthenticator(t, store, now)
	request := &blogopenv1.GetUserInfoRequest{UserId: 42}
	timestamp := strconv.FormatInt(now.Unix(), 10)
	nonce := "nonce-1234567890"
	signature, err := BuildHMACSignature([]byte(testHMACSecret), publicUserInfoMethod, testAccessKey, timestamp, nonce, request)
	if err != nil {
		t.Fatalf("BuildHMACSignature() error = %v", err)
	}
	ctx := hmacContext(testAccessKey, signature, timestamp, nonce)
	caller, err := invokeAuth(t, authenticator, ctx, request, publicUserInfoMethod)
	if err != nil {
		t.Fatalf("first HMAC request error = %v", err)
	}
	if caller.Kind != GRPCCallerExternal || caller.ID != testAccessKey {
		t.Fatalf("caller = %#v", caller)
	}
	if store.lastTTL != time.Minute {
		t.Fatalf("nonce ttl = %v", store.lastTTL)
	}
	_, err = invokeAuth(t, authenticator, ctx, request, publicUserInfoMethod)
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("replayed request code = %v, want %v", got, codes.Unauthenticated)
	}
}

// TestGRPCHMACRejectsTimeoutBadSignatureAndBodyTampering 验证时间、签名和请求体边界。
func TestGRPCHMACRejectsTimeoutBadSignatureAndBodyTampering(t *testing.T) {
	// 1. 构造超时、未来时间、错误密钥和请求体篡改场景
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	request := &blogopenv1.GetUserInfoRequest{UserId: 42}
	tests := []struct {
		name      string                         // name 是测试场景名称。
		timestamp string                         // timestamp 是待签名时间戳。
		nonce     string                         // nonce 是当前场景的唯一随机值。
		signed    *blogopenv1.GetUserInfoRequest // signed 是生成签名时使用的请求。
		request   *blogopenv1.GetUserInfoRequest // request 是服务端实际收到的请求。
		secret    string                         // secret 是生成签名使用的密钥。
	}{
		{name: "达到时间窗口边界", timestamp: strconv.FormatInt(now.Add(-60*time.Second).Unix(), 10), nonce: "nonce-timeout-123", signed: request, request: request, secret: testHMACSecret},
		{name: "未来时间", timestamp: strconv.FormatInt(now.Add(time.Second).Unix(), 10), nonce: "nonce-future-1234", signed: request, request: request, secret: testHMACSecret},
		{name: "错误签名", timestamp: strconv.FormatInt(now.Unix(), 10), nonce: "nonce-badsign-123", signed: request, request: request, secret: "another-secret-that-is-at-least-32-bytes"},
		{name: "请求体被篡改", timestamp: strconv.FormatInt(now.Unix(), 10), nonce: "nonce-bodyhash-12", signed: request, request: &blogopenv1.GetUserInfoRequest{UserId: 43}, secret: testHMACSecret},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator := testAuthenticator(t, &memoryNonceStore{}, now)
			signature, err := BuildHMACSignature([]byte(test.secret), publicUserInfoMethod, testAccessKey, test.timestamp, test.nonce, test.signed)
			if err != nil {
				t.Fatalf("BuildHMACSignature() error = %v", err)
			}
			_, err = invokeAuth(t, authenticator, hmacContext(testAccessKey, signature, test.timestamp, test.nonce), test.request, publicUserInfoMethod)
			if got := status.Code(err); got != codes.Unauthenticated {
				t.Fatalf("status.Code() = %v, want %v; err = %v", got, codes.Unauthenticated, err)
			}
		})
	}
}

// TestGRPCAuthenticationUsesStandardCodesAndDoesNotExposeCredentials 验证状态与 Leo 日志均不泄漏凭据。
func TestGRPCAuthenticationUsesStandardCodesAndDoesNotExposeCredentials(t *testing.T) {
	// 1. 将 Leo 全局日志切换到测试文件并在结束后恢复
	logPath := filepath.Join(t.TempDir(), "grpc-auth.log")
	previousLogger := leolog.L()
	leolog.SetLogger(leoslog.New(leoslog.LevelAdapt(leolog.LevelDebug), leoslog.File(logPath, 1, 1, 1), leoslog.Console(false)))
	t.Cleanup(func() { leolog.SetLogger(previousLogger) })

	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	store := &memoryNonceStore{err: errors.New("redis connection includes nonce-sensitive-value")}
	authenticator := testAuthenticator(t, store, now)
	request := &blogopenv1.GetUserInfoRequest{UserId: 42}
	timestamp := strconv.FormatInt(now.Unix(), 10)
	nonce := "nonce-sensitive-value"
	signature, err := BuildHMACSignature([]byte(testHMACSecret), publicUserInfoMethod, testAccessKey, timestamp, nonce, request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = invokeAuth(t, authenticator, hmacContext(testAccessKey, signature, timestamp, nonce), request, publicUserInfoMethod)
	if got := status.Code(err); got != codes.Unavailable {
		t.Fatalf("status.Code() = %v, want %v", got, codes.Unavailable)
	}

	// 2. 再执行包含完整 JWT 的失败请求，覆盖两类凭据的日志边界
	jwtToken := signJWT(t, testJWTSecret, "untrusted-issuer", "article-service", now.Add(-time.Minute), now.Add(time.Minute), jwt.SigningMethodHS256)
	jwtContext := metadata.NewIncomingContext(context.Background(), metadata.Pairs(metadataAuthorization, "Bearer "+jwtToken))
	_, jwtErr := invokeAuth(t, authenticator, jwtContext, request, userBasicInfoMethod)
	if got := status.Code(jwtErr); got != codes.Unauthenticated {
		t.Fatalf("JWT status.Code() = %v, want %v", got, codes.Unauthenticated)
	}

	// 3. 读取 Leo 日志并确认响应与日志都不包含任何认证材料
	logBytes, readErr := os.ReadFile(logPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("read auth log: %v", readErr)
	}
	logOutput := string(logBytes)
	for _, sensitive := range []string{testJWTSecret, jwtToken, testHMACSecret, signature, nonce, "redis connection"} {
		if strings.Contains(err.Error(), sensitive) || strings.Contains(jwtErr.Error(), sensitive) {
			t.Fatalf("authentication error exposed sensitive value %q: hmac=%v jwt=%v", sensitive, err, jwtErr)
		}
		if strings.Contains(logOutput, sensitive) {
			t.Fatalf("authentication log exposed sensitive value %q: %s", sensitive, logOutput)
		}
	}
}

// TestGRPCAuthenticationEnforcesRPCAuthType 验证内部与外部调用方只能访问对应 RPC。
func TestGRPCAuthenticationEnforcesRPCAuthType(t *testing.T) {
	// 1. 验证内部 JWT 不能调用外部合作方 RPC
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	authenticator := testAuthenticator(t, &memoryNonceStore{}, now)
	request := &blogopenv1.GetUserInfoRequest{UserId: 42}
	token := signJWT(t, testJWTSecret, testIssuer, "article-service", now.Add(-time.Minute), now.Add(time.Minute), jwt.SigningMethodHS256)
	jwtCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(metadataAuthorization, "Bearer "+token))
	_, err := invokeAuth(t, authenticator, jwtCtx, request, publicUserInfoMethod)
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("internal caller on external RPC code = %v, want %v", got, codes.PermissionDenied)
	}

	timestamp := strconv.FormatInt(now.Unix(), 10)
	nonce := "nonce-wrong-rpc-12"
	signature, err := BuildHMACSignature([]byte(testHMACSecret), userBasicInfoMethod, testAccessKey, timestamp, nonce, request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = invokeAuth(t, authenticator, hmacContext(testAccessKey, signature, timestamp, nonce), request, userBasicInfoMethod)
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("external caller on internal RPC code = %v, want %v", got, codes.PermissionDenied)
	}
}

// TestGRPCAuthenticationLeavesLegacyRPCsUnchanged 验证存量示例 RPC 不受新认证策略影响。
func TestGRPCAuthenticationLeavesLegacyRPCsUnchanged(t *testing.T) {
	// 1. 无凭据调用存量 RPC 时应直接进入原 Handler
	authenticator := testAuthenticator(t, &memoryNonceStore{}, time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	handled := false
	_, err := authenticator.UnaryServerInterceptor()(context.Background(), "legacy-request", &grpc.UnaryServerInfo{FullMethod: "/book.Book/ShowBook"}, func(context.Context, any) (any, error) {
		handled = true
		return nil, nil
	})
	if err != nil || !handled {
		t.Fatalf("legacy RPC changed: handled=%v err=%v", handled, err)
	}
}

// hmacContext 创建携带完整外部 HMAC Metadata 的请求上下文。
func hmacContext(accessKeyID, signature, timestamp, nonce string) context.Context {
	// 1. 使用 gRPC 小写 Metadata Key 构造外部认证上下文
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		metadataAccessKeyID, accessKeyID,
		metadataSignature, signature,
		metadataTimestamp, timestamp,
		metadataNonce, nonce,
	))
}

// TestProvideGRPCAuthSettingsValidatesRequiredConfiguration 验证认证配置默认值和密钥强度。
func TestProvideGRPCAuthSettingsValidatesRequiredConfiguration(t *testing.T) {
	// 1. 验证完整配置转换为一分钟时间窗口和 Nonce TTL
	config := &conf.Config{Server: &conf.Server{Grpc: &conf.GRPC{Auth: &conf.GRPCAuth{
		JwtIssuer:             testIssuer,
		JwtSecret:             testJWTSecret,
		HmacTimeWindowSeconds: 60,
		NonceTtlSeconds:       60,
		HmacAccessKeys:        map[string]string{testAccessKey: testHMACSecret},
	}}}}
	settings, err := ProvideGRPCAuthSettings(config)
	if err != nil {
		t.Fatalf("ProvideGRPCAuthSettings() error = %v", err)
	}
	if settings.JWTIssuer != testIssuer || settings.HMACWindow != time.Minute || settings.NonceTTL != time.Minute {
		t.Fatalf("settings = %#v", settings)
	}

	config.Server.Grpc.Auth.JwtSecret = "short"
	if _, err := ProvideGRPCAuthSettings(config); err == nil || strings.Contains(err.Error(), "short") {
		t.Fatalf("short secret validation error = %v", err)
	}
}
