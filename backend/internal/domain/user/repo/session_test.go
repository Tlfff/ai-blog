package repo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"github.com/redis/go-redis/v9"
)

// TestSessionRepositoryCreateAndDelete 验证会话内容、TTL 和单设备删除命令。
func TestSessionRepositoryCreateAndDelete(t *testing.T) {
	t.Parallel()

	writer := &fakeSessionWriter{}
	repository := &SessionRepository{client: writer, writer: writer}
	session := user.Session{UserID: 7, Role: user.RoleUser, Device: "web", LoginIP: "203.0.113.8", LoginTime: 1_700_000_000}

	// 1. 创建会话时保存完整登录信息和七天 TTL
	if err := repository.Create(context.Background(), "web-token", session, int64((7 * 24 * time.Hour).Seconds())); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	var stored user.Session
	if err := json.Unmarshal(writer.payload, &stored); err != nil {
		t.Fatalf("decode stored session: %v", err)
	}
	if writer.setKey != authTokenKeyPrefix+"web-token" || writer.ttl != 7*24*time.Hour || stored.Device != "web" || stored.LoginIP != session.LoginIP {
		t.Fatalf("stored session = %#v key=%q ttl=%v", stored, writer.setKey, writer.ttl)
	}

	// 2. 删除时只移除当前 Token 和对应集合成员
	if err := repository.Delete(context.Background(), "web-token", 7); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if writer.deletedKey != authTokenKeyPrefix+"web-token" || writer.removedMember != "web-token" {
		t.Fatalf("delete key=%q removed=%q", writer.deletedKey, writer.removedMember)
	}
}

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

type fakeSessionWriter struct {
	redis.Pipeliner               // Pipeliner 提供未使用命令的接口占位实现。
	payload         []byte        // payload 是写入的会话 JSON。
	setKey          string        // setKey 是写入的 Token Key。
	ttl             time.Duration // ttl 是 Token 会话有效期。
	deletedKey      string        // deletedKey 是删除的 Token Key。
	removedMember   string        // removedMember 是从用户集合移除的 Token。
}

// Get 返回空查询结果，本测试不使用读取路径。
func (f *fakeSessionWriter) Get(context.Context, string) *redis.StringCmd {
	// 1. 返回会话不存在，本测试只验证写入路径
	return redis.NewStringResult("", redis.Nil)
}

// Eval 返回测试用 Redis 脚本结果。
func (f *fakeSessionWriter) Eval(context.Context, string, []string, ...interface{}) *redis.Cmd {
	// 1. 模拟会话收敛 Lua 脚本执行成功
	return redis.NewCmdResult(int64(1), nil)
}

// TxPipeline 返回记录命令的测试流水线。
func (f *fakeSessionWriter) TxPipeline() redis.Pipeliner {
	// 1. 返回自身以记录流水线命令
	return f
}

// Set 记录 Token 会话内容和 TTL。
func (f *fakeSessionWriter) Set(_ context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	// 1. 记录会话 Key、数据和有效期
	f.setKey, f.ttl = key, ttl
	payload, ok := value.([]byte)
	if !ok {
		return redis.NewStatusResult("", errors.New("会话测试收到非字节数据"))
	}
	f.payload = payload
	return redis.NewStatusResult("OK", nil)
}

// SAdd 接受用户 Token 集合写入命令。
func (f *fakeSessionWriter) SAdd(context.Context, string, ...interface{}) *redis.IntCmd {
	// 1. 模拟用户 Token 集合写入成功
	return redis.NewIntResult(1, nil)
}

// Del 记录待删除的 Token Key。
func (f *fakeSessionWriter) Del(_ context.Context, keys ...string) *redis.IntCmd {
	// 1. 记录当前设备 Token Key
	if len(keys) > 0 {
		f.deletedKey = keys[0]
	}
	return redis.NewIntResult(1, nil)
}

// SRem 记录从用户集合移除的 Token。
func (f *fakeSessionWriter) SRem(_ context.Context, _ string, members ...interface{}) *redis.IntCmd {
	// 1. 记录从用户 Token 集合移除的成员
	if len(members) > 0 {
		member, ok := members[0].(string)
		if !ok {
			return redis.NewIntResult(0, errors.New("会话测试收到非字符串 Token"))
		}
		f.removedMember = member
	}
	return redis.NewIntResult(1, nil)
}

// Exec 表示记录的流水线命令执行成功。
func (f *fakeSessionWriter) Exec(context.Context) ([]redis.Cmder, error) {
	// 1. 模拟 Redis 事务流水线执行成功
	return nil, nil
}
