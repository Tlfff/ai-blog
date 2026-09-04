package repo

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"github.com/redis/go-redis/v9"
)

// setNXClient 定义 Redis 原子防重写入所需的最小能力。
type setNXClient interface {
	// SetNX 原子写入带有效期的防重复键。
	SetNX(context.Context, string, interface{}, time.Duration) *redis.BoolCmd
}

// SubmissionGuard 使用 Redis 保存跨实例共享的短期提交指纹。
type SubmissionGuard struct {
	client setNXClient // client 提供带 TTL 的原子 SET NX 能力。
}

// NewSubmissionGuard 创建文章创建防重复仓储。
func NewSubmissionGuard(client clients.RedisClient) *SubmissionGuard {
	// 1. 启动阶段拒绝缺少 Redis 客户端
	if client == nil {
		panic("文章防重复仓储缺少 Redis 客户端")
	}
	return &SubmissionGuard{client: client}
}

// Acquire 原子占用提交指纹并由 Redis TTL 自动回收。
func (g *SubmissionGuard) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	// 1. SET NX 保证多个进程或实例只能有一个请求占用成功
	return g.client.SetNX(ctx, key, "1", ttl).Result()
}
