package middleware

import "github.com/google/wire"

// GRPCProviderSet 提供 gRPC 认证配置、认证器与 Unary Interceptor。
var GRPCProviderSet = wire.NewSet(
	ProvideGRPCAuthSettings,
	NewGRPCAuthenticator,
	ProvideGRPCUnaryServerInterceptor,
)
