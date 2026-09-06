package middleware

import "context"

// GRPCCallerKind 表示开放 gRPC 请求使用的认证类型。
type GRPCCallerKind uint8

const (
	// GRPCCallerInternal 表示通过内部 HS256 JWT 认证的调用方。
	GRPCCallerInternal GRPCCallerKind = iota + 1
	// GRPCCallerExternal 表示通过外部 HMAC-SHA256 认证的合作方。
	GRPCCallerExternal
)

// GRPCCaller 是认证成功后注入请求上下文的调用方身份。
type GRPCCaller struct {
	Kind GRPCCallerKind // Kind 是调用方认证类型。
	ID   string         // ID 是 JWT subject 或外部 Access Key ID。
}

type grpcCallerContextKey struct{}

// WithGRPCCaller 将认证后的调用方身份写入上下文。
func WithGRPCCaller(ctx context.Context, caller GRPCCaller) context.Context {
	// 1. 使用私有键保存身份，避免与业务上下文键冲突
	return context.WithValue(ctx, grpcCallerContextKey{}, caller)
}

// GRPCCallerFromContext 读取认证后的调用方身份。
func GRPCCallerFromContext(ctx context.Context) (GRPCCaller, bool) {
	// 1. 类型断言确保未认证请求不能伪造调用方身份
	caller, ok := ctx.Value(grpcCallerContextKey{}).(GRPCCaller)
	return caller, ok
}
