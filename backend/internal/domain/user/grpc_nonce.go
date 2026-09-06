package user

import (
	"context"
	"time"
)

// GRPCNonceStore 定义开放 gRPC 外部请求所需的原子防重放能力。
type GRPCNonceStore interface {
	// ReserveGRPCNonce 原子占用 Access Key 下的 Nonce；已存在时返回 false。
	ReserveGRPCNonce(ctx context.Context, accessKeyID, nonce string, ttl time.Duration) (bool, error)
}
