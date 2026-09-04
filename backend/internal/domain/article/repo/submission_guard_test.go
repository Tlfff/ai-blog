package repo

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// fakeSetNXClient 记录 Redis SET NX 请求。
type fakeSetNXClient struct {
	key string        // key 是提交指纹。
	ttl time.Duration // ttl 是自动回收时间。
}

// SetNX 返回占用成功并记录原子操作参数。
func (f *fakeSetNXClient) SetNX(_ context.Context, key string, _ interface{}, ttl time.Duration) *redis.BoolCmd {
	// 1. 记录跨实例防重键和 TTL
	f.key, f.ttl = key, ttl
	return redis.NewBoolResult(true, nil)
}

// TestSubmissionGuardUsesAtomicSetNXWithTTL 验证 Redis 原子防重和自动过期。
func TestSubmissionGuardUsesAtomicSetNXWithTTL(t *testing.T) {
	// 1. 调用防重仓储占用两秒键
	client := &fakeSetNXClient{}
	guard := &SubmissionGuard{client: client}
	acquired, err := guard.Acquire(context.Background(), "article:create:test", 2*time.Second)
	if err != nil || !acquired || client.key != "article:create:test" || client.ttl != 2*time.Second {
		t.Fatalf("acquired = %v, error = %v, key = %q, ttl = %s", acquired, err, client.key, client.ttl)
	}
}
