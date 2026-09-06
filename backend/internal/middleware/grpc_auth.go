package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	metadataAuthorization = "authorization"
	metadataAccessKeyID   = "x-access-key-id"
	metadataSignature     = "x-signature"
	metadataTimestamp     = "x-timestamp"
	metadataNonce         = "x-nonce"

	userBasicInfoMethod  = blogopenv1.UserService_GetUserBasicInfo_FullMethodName
	publicUserInfoMethod = blogopenv1.UserService_GetPublicUserInfo_FullMethodName

	defaultHMACTimeWindow = 60 * time.Second
	minimumSecretBytes    = 32
)

// GRPCAuthSettings 是开放 gRPC 认证所需的不可变配置。
type GRPCAuthSettings struct {
	JWTIssuer     string            // JWTIssuer 是内部 JWT 的唯一受信签发者。
	JWTSecret     []byte            // JWTSecret 是 HS256 对称密钥。
	JWTClockSkew  time.Duration     // JWTClockSkew 是内部服务时钟偏差容忍值。
	HMACWindow    time.Duration     // HMACWindow 是外部请求允许的最大历史时间。
	NonceTTL      time.Duration     // NonceTTL 是 Redis 防重放记录有效期。
	HMACAccessKey map[string][]byte // HMACAccessKey 按 Access Key ID 保存合作方密钥。
}

// ProvideGRPCAuthSettings 从统一配置构造并校验 gRPC 认证设置。
func ProvideGRPCAuthSettings(config *conf.Config) (GRPCAuthSettings, error) {
	// 1. 启动阶段拒绝缺失的认证配置，避免服务无认证运行
	if config == nil || config.GetServer() == nil || config.GetServer().GetGrpc() == nil || config.GetServer().GetGrpc().GetAuth() == nil {
		return GRPCAuthSettings{}, errors.New("gRPC 认证配置缺失")
	}

	// 2. 读取认证参数并为时间窗口设置兼容默认值
	auth := config.GetServer().GetGrpc().GetAuth()
	settings := GRPCAuthSettings{
		JWTIssuer:     strings.TrimSpace(auth.GetJwtIssuer()),
		JWTSecret:     []byte(auth.GetJwtSecret()),
		JWTClockSkew:  time.Duration(auth.GetJwtClockSkewSeconds()) * time.Second,
		HMACWindow:    time.Duration(auth.GetHmacTimeWindowSeconds()) * time.Second,
		NonceTTL:      time.Duration(auth.GetNonceTtlSeconds()) * time.Second,
		HMACAccessKey: make(map[string][]byte, len(auth.GetHmacAccessKeys())),
	}
	if settings.HMACWindow == 0 {
		settings.HMACWindow = defaultHMACTimeWindow
	}
	if settings.NonceTTL == 0 {
		settings.NonceTTL = defaultHMACTimeWindow
	}

	// 3. 复制合作方密钥，避免保留可变配置 Map 的引用
	for accessKeyID, secret := range auth.GetHmacAccessKeys() {
		settings.HMACAccessKey[accessKeyID] = []byte(secret)
	}

	// 4. 在进程启动前完成密钥强度与时间约束校验
	if err := validateGRPCAuthSettings(settings); err != nil {
		return GRPCAuthSettings{}, err
	}
	return settings, nil
}

// validateGRPCAuthSettings 校验内部 JWT 与外部 HMAC 的最低安全配置。
func validateGRPCAuthSettings(settings GRPCAuthSettings) error {
	// 1. 校验内部 JWT 的签发者和 HS256 密钥强度
	if settings.JWTIssuer == "" {
		return errors.New("gRPC JWT issuer 不能为空")
	}
	if len(settings.JWTSecret) < minimumSecretBytes {
		return fmt.Errorf("gRPC JWT 密钥长度不能少于 %d 字节", minimumSecretBytes)
	}

	// 2. 校验防重放时间关系及每个合作方密钥
	if settings.JWTClockSkew < 0 || settings.HMACWindow <= 0 || settings.NonceTTL < settings.HMACWindow {
		return errors.New("gRPC 认证时间配置无效")
	}
	if len(settings.HMACAccessKey) == 0 {
		return errors.New("gRPC HMAC Access Key 不能为空")
	}
	for accessKeyID, secret := range settings.HMACAccessKey {
		if !validCanonicalField(accessKeyID, 1, 128) {
			return errors.New("gRPC HMAC Access Key ID 无效")
		}
		if len(secret) < minimumSecretBytes {
			return fmt.Errorf("gRPC HMAC 密钥长度不能少于 %d 字节", minimumSecretBytes)
		}
	}
	return nil
}

// GRPCAuthenticator 校验内部 JWT 或外部 HMAC，并注入调用方身份。
type GRPCAuthenticator struct {
	settings GRPCAuthSettings          // settings 是已完成启动校验的认证配置副本。
	nonces   userdomain.GRPCNonceStore // nonces 原子拒绝同一合作方重复使用 Nonce。
	now      func() time.Time          // now 提供可测试的当前时间。
}

// NewGRPCAuthenticator 创建 gRPC 统一认证器。
func NewGRPCAuthenticator(settings GRPCAuthSettings, nonces userdomain.GRPCNonceStore) (*GRPCAuthenticator, error) {
	// 1. 重复校验直接构造时传入的认证配置
	if err := validateGRPCAuthSettings(settings); err != nil {
		return nil, err
	}
	if nonces == nil {
		return nil, errors.New("gRPC Nonce 存储缺失")
	}

	// 2. 深拷贝所有密钥，防止调用方在运行期修改认证材料
	settings.JWTSecret = append([]byte(nil), settings.JWTSecret...)
	keys := make(map[string][]byte, len(settings.HMACAccessKey))
	for accessKeyID, secret := range settings.HMACAccessKey {
		keys[accessKeyID] = append([]byte(nil), secret...)
	}
	settings.HMACAccessKey = keys

	// 3. 使用系统时钟创建认证器，测试可替换 now
	return &GRPCAuthenticator{settings: settings, nonces: nonces, now: time.Now}, nil
}

// UnaryServerInterceptor 返回接入 Leo gRPC Server 的统一认证拦截器。
func (a *GRPCAuthenticator) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	// 1. 仅拦截本工单新增的开放用户 RPC，保持存量示例接口兼容
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// 1.1 未配置认证策略的方法直接沿用原处理链
		required := requiredCallerKind(info.FullMethod)
		if required == 0 {
			return handler(ctx, req)
		}
		// 1.2 校验凭据并限制内部、外部调用方只能访问对应 RPC
		caller, authErr := a.authenticate(ctx, req, info.FullMethod)
		if authErr != nil {
			return nil, status.Error(authErr.code, authErr.message)
		}
		if caller.Kind != required {
			return nil, status.Error(codes.PermissionDenied, "调用方无权访问该 RPC")
		}

		// 1.3 注入不含凭据的调用方身份后执行生成服务
		return handler(WithGRPCCaller(ctx, caller), req)
	}
}

// ProvideGRPCUnaryServerInterceptor 向 Wire 暴露 gRPC Unary Interceptor。
func ProvideGRPCUnaryServerInterceptor(authenticator *GRPCAuthenticator) grpc.UnaryServerInterceptor {
	// 1. 向 Leo Server 提供唯一的开放用户认证拦截器
	return authenticator.UnaryServerInterceptor()
}

// grpcAuthError 保存可安全返回客户端的标准状态，不包含底层凭据或依赖错误。
type grpcAuthError struct {
	code    codes.Code // code 是标准 gRPC 状态码。
	message string     // message 是不包含敏感信息的固定提示。
}

// authenticate 根据 Metadata 类型选择内部 JWT 或外部 HMAC 认证。
func (a *GRPCAuthenticator) authenticate(ctx context.Context, req any, fullMethod string) (GRPCCaller, *grpcAuthError) {
	// 1. 读取传入 Metadata，并拒绝同时携带两类凭据的歧义请求
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return GRPCCaller{}, unauthenticated()
	}
	hasExternalMetadata := hasAnyMetadata(md, metadataAccessKeyID, metadataSignature, metadataTimestamp, metadataNonce)
	hasAuthorization := len(md.Get(metadataAuthorization)) > 0
	if hasExternalMetadata && hasAuthorization {
		return GRPCCaller{}, unauthenticated()
	}

	// 2. 任何外部认证字段都进入完整 HMAC 校验，否则要求 Bearer JWT
	if hasExternalMetadata {
		return a.authenticateHMAC(ctx, md, req, fullMethod)
	}
	return a.authenticateJWT(md)
}

// authenticateJWT 校验 HS256 算法、签发者、签名、时间和必要声明。
func (a *GRPCAuthenticator) authenticateJWT(md metadata.MD) (GRPCCaller, *grpcAuthError) {
	// 1. 要求唯一且格式严格的 Bearer Token
	authorization, ok := singleMetadata(md, metadataAuthorization)
	if !ok || !strings.HasPrefix(authorization, "Bearer ") {
		return GRPCCaller{}, unauthenticated()
	}
	tokenString := strings.TrimPrefix(authorization, "Bearer ")
	if tokenString == "" || strings.TrimSpace(tokenString) != tokenString {
		return GRPCCaller{}, unauthenticated()
	}

	// 2. 固定 HS256，并由 jwt 库校验 issuer、exp、iat 和签名
	claims := new(jwt.RegisteredClaims)
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return a.settings.JWTSecret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(a.settings.JWTIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(a.settings.JWTClockSkew),
		jwt.WithTimeFunc(a.now),
		jwt.WithStrictDecoding(),
	)
	// 3. 强制 sub、iat、exp 存在且时间顺序有效
	if err != nil || token == nil || !token.Valid || claims.Subject == "" || claims.IssuedAt == nil || claims.ExpiresAt == nil || !claims.ExpiresAt.After(claims.IssuedAt.Time) {
		return GRPCCaller{}, unauthenticated()
	}
	if !validCanonicalField(claims.Subject, 1, 128) {
		return GRPCCaller{}, unauthenticated()
	}

	// 4. 只注入 subject，不传播 JWT 或签名材料
	return GRPCCaller{Kind: GRPCCallerInternal, ID: claims.Subject}, nil
}

// authenticateHMAC 校验合作方凭据、请求内容、时间窗口和 Nonce 防重放。
func (a *GRPCAuthenticator) authenticateHMAC(ctx context.Context, md metadata.MD, req any, fullMethod string) (GRPCCaller, *grpcAuthError) {
	// 1. 要求四个 Metadata 字段唯一存在且可安全进入规范串
	accessKeyID, keyOK := singleMetadata(md, metadataAccessKeyID)
	signature, signatureOK := singleMetadata(md, metadataSignature)
	timestampValue, timestampOK := singleMetadata(md, metadataTimestamp)
	nonce, nonceOK := singleMetadata(md, metadataNonce)
	if !keyOK || !signatureOK || !timestampOK || !nonceOK || !validCanonicalField(accessKeyID, 1, 128) || !validCanonicalField(nonce, 16, 128) {
		return GRPCCaller{}, unauthenticated()
	}

	// 2. 查询合作方密钥并校验 Unix 秒处于尚未过期的历史窗口
	secret, ok := a.settings.HMACAccessKey[accessKeyID]
	if !ok {
		return GRPCCaller{}, unauthenticated()
	}
	timestamp, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil || timestamp <= 0 || strconv.FormatInt(timestamp, 10) != timestampValue {
		return GRPCCaller{}, unauthenticated()
	}
	now := a.now().Unix()
	windowSeconds := int64(a.settings.HMACWindow / time.Second)
	if timestamp > now || now-timestamp >= windowSeconds {
		return GRPCCaller{}, unauthenticated()
	}

	// 3. 对完整方法和确定性请求体摘要计算预期签名并恒定时间比较
	expected, err := hmacSignature(secret, fullMethod, accessKeyID, timestampValue, nonce, req)
	if err != nil {
		return GRPCCaller{}, &grpcAuthError{code: codes.Internal, message: "无法校验请求认证"}
	}
	provided, err := hex.DecodeString(signature)
	if err != nil || len(provided) != sha256.Size || !hmac.Equal(provided, expected) {
		return GRPCCaller{}, unauthenticated()
	}

	// 4. 仅在签名成功后原子占用 Nonce，防止无效请求抢占
	reserved, err := a.nonces.ReserveGRPCNonce(ctx, accessKeyID, nonce, a.settings.NonceTTL)
	if err != nil {
		return GRPCCaller{}, &grpcAuthError{code: codes.Unavailable, message: "认证服务暂不可用"}
	}
	if !reserved {
		return GRPCCaller{}, unauthenticated()
	}

	// 5. 只注入 Access Key ID，不传播密钥、签名或 Nonce
	return GRPCCaller{Kind: GRPCCallerExternal, ID: accessKeyID}, nil
}

// BuildHMACSignature 按服务端规范生成十六进制 HMAC-SHA256 签名。
//
// 规范串依次包含完整 RPC 方法、Access Key ID、Unix 秒时间戳、Nonce、
// 确定性 protobuf 请求体的 SHA256，每项之间使用一个换行符分隔。
//
// 参数说明：
//   - secret：合作方 HMAC 密钥，部署时至少 32 字节。
//   - fullMethod：包含包名、服务名和方法名的完整 gRPC 方法。
//   - accessKeyID：合作方 Access Key ID。
//   - timestamp：规范十进制 Unix 秒时间戳。
//   - nonce：本次请求唯一的随机值。
//   - request：待签名的 protobuf 请求消息。
func BuildHMACSignature(secret []byte, fullMethod, accessKeyID, timestamp, nonce string, request proto.Message) (string, error) {
	// 1. 按与服务端相同的规范串计算原始 HMAC
	signature, err := hmacSignature(secret, fullMethod, accessKeyID, timestamp, nonce, request)
	if err != nil {
		return "", err
	}

	// 2. 使用十六进制编码形成 x-signature Metadata 值
	return hex.EncodeToString(signature), nil
}

// hmacSignature 计算绑定方法、调用方、时间、Nonce 和请求体的原始签名。
//
// 参数说明：
//   - secret：合作方 HMAC 密钥。
//   - fullMethod：完整 gRPC 方法。
//   - accessKeyID：合作方 Access Key ID。
//   - timestamp：规范十进制 Unix 秒时间戳。
//   - nonce：本次请求唯一随机值。
//   - request：待签名请求，必须实现 proto.Message。
func hmacSignature(secret []byte, fullMethod, accessKeyID, timestamp, nonce string, request any) ([]byte, error) {
	// 1. 使用确定性 protobuf 编码固定同一请求的字节表示
	message, ok := request.(proto.Message)
	if !ok || message == nil {
		return nil, errors.New("gRPC 请求不是 protobuf 消息")
	}
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil, err
	}

	// 2. 将请求体摘要与关键 Metadata 组成无歧义规范串
	bodyDigest := sha256.Sum256(body)
	canonical := strings.Join([]string{fullMethod, accessKeyID, timestamp, nonce, hex.EncodeToString(bodyDigest[:])}, "\n")

	// 3. 使用标准库 HMAC-SHA256 计算原始签名字节
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return mac.Sum(nil), nil
}

// requiredCallerKind 返回开放用户 RPC 要求的调用方认证类型。
func requiredCallerKind(fullMethod string) GRPCCallerKind {
	// 1. 只为本工单新增的两个 RPC 声明认证策略
	switch fullMethod {
	case userBasicInfoMethod:
		return GRPCCallerInternal
	case publicUserInfoMethod:
		return GRPCCallerExternal
	default:
		return 0
	}
}

// hasAnyMetadata 判断请求是否携带任一指定 Metadata。
func hasAnyMetadata(md metadata.MD, keys ...string) bool {
	// 1. 任一字段出现即按对应认证类型执行完整校验
	for _, key := range keys {
		if len(md.Get(key)) > 0 {
			return true
		}
	}
	return false
}

// singleMetadata 读取唯一且非空的 Metadata 值。
func singleMetadata(md metadata.MD, key string) (string, bool) {
	// 1. 重复值和空值均视为认证格式错误
	values := md.Get(key)
	returnValue := ""
	if len(values) == 1 {
		returnValue = values[0]
	}
	return returnValue, len(values) == 1 && returnValue != ""
}

// validCanonicalField 校验规范串字段长度及可见 ASCII 约束。
func validCanonicalField(value string, minimum, maximum int) bool {
	// 1. 拒绝空白、控制字符和可能破坏换行分隔的内容
	if len(value) < minimum || len(value) > maximum {
		return false
	}
	for index := range value {
		if value[index] <= 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

// unauthenticated 返回不暴露失败细节和凭据的固定认证错误。
func unauthenticated() *grpcAuthError {
	// 1. 所有无效凭据统一映射为 Unauthenticated
	return &grpcAuthError{code: codes.Unauthenticated, message: "gRPC 认证失败"}
}
