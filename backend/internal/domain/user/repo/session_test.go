package repo

import (
	"context"
	"errors"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"github.com/redis/go-redis/v9"
)

// TestSessionRepositoryFindByToken 验证有效、缺失和损坏会话的查询行为。
func TestSessionRepositoryFindByToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string // name 是测试场景名称。
		payload   string // payload 是 Redis 会话 JSON。
		redisErr  error  // redisErr 是预设 Redis 错误。
		wantID    uint64 // wantID 是预期用户标识。
		wantError error  // wantError 是预期领域错误。
	}{
		{name: "有效会话", payload: `{"user_id":7,"role":1}`, wantID: 7},
		{name: "会话不存在", redisErr: redis.Nil, wantError: user.ErrSessionNotFound},
		{name: "会话缺少用户", payload: `{"role":1}`, wantError: user.ErrSessionNotFound},
		{name: "会话 JSON 损坏", payload: `{`, wantError: errInvalidSessionJSON},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 1. 使用 Redis 命令结果替身执行会话查询
			repository := &SessionRepository{client: &fakeSessionGetter{payload: tt.payload, err: tt.redisErr}}
			session, err := repository.FindByToken(context.Background(), "token")

			// 2. 验证稳定领域错误或有效身份
			if tt.wantError != nil {
				if tt.wantError == errInvalidSessionJSON {
					if err == nil || errors.Is(err, user.ErrSessionNotFound) {
						t.Fatalf("FindByToken() error = %v, want JSON parse error", err)
					}
					return
				}
				if !errors.Is(err, tt.wantError) {
					t.Fatalf("FindByToken() error = %v, want %v", err, tt.wantError)
				}
				return
			}
			if err != nil || session.UserID != tt.wantID {
				t.Fatalf("FindByToken() session = %#v error = %v", session, err)
			}
		})
	}
}

var errInvalidSessionJSON = errors.New("测试标记：损坏会话 JSON")

type fakeSessionGetter struct {
	payload string // payload 是预设 Redis 字符串。
	err     error  // err 是预设 Redis 错误。
}

// Get 返回测试预设的 Redis 字符串命令结果。
func (f *fakeSessionGetter) Get(context.Context, string) *redis.StringCmd {
	// 1. 构造与 go-redis 客户端一致的命令结果
	return redis.NewStringResult(f.payload, f.err)
}
