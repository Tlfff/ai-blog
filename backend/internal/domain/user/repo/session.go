package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"github.com/redis/go-redis/v9"
)

const (
	authTokenKeyPrefix      = "auth:token:"
	authUserTokensKeyPrefix = "auth:user-tokens:"
	passwordChangeKeyPrefix = "user:password-change:"
)

type sessionGetter interface {
	// Get 读取指定会话 Key。
	Get(context.Context, string) *redis.StringCmd
}

// passwordChangeClient 定义改密凭证写入、原子消费和恢复所需的 Redis 能力。
type passwordChangeClient interface {
	// Set 保存带有效期的改密凭证。
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
	// Eval 原子执行改密凭证消费脚本。
	Eval(context.Context, string, []string, ...interface{}) *redis.Cmd
}

// sessionWriter 定义会话创建和删除所需的 Redis 原子流水线能力。
type sessionWriter interface {
	sessionGetter
	// Set 保存带有效期的登录会话。
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
	// Del 删除指定登录会话或集合成员。
	Del(context.Context, ...string) *redis.IntCmd
	// SAdd 将 Token 加入用户会话集合。
	SAdd(context.Context, string, ...interface{}) *redis.IntCmd
	// SRem 将 Token 从用户会话集合移除。
	SRem(context.Context, string, ...interface{}) *redis.IntCmd
	// Eval 原子执行多设备会话收敛脚本。
	Eval(context.Context, string, []string, ...interface{}) *redis.Cmd
	// TxPipeline 创建会话写入事务流水线。
	TxPipeline() redis.Pipeliner
}

// SessionRepository 使用 Redis 保存用户登录会话。
type SessionRepository struct {
	client         sessionGetter        // client 提供登录会话查询能力。
	writer         sessionWriter        // writer 提供登录会话原子写入能力。
	passwordClient passwordChangeClient // passwordClient 提供改密凭证原子消费能力。
}

// NewSessionRepository 创建用户会话 Redis 仓储。
func NewSessionRepository(client clients.RedisClient) *SessionRepository {
	// 1. 启动阶段拒绝缺少 Redis 客户端的会话仓储
	if client == nil {
		panic("用户会话仓储缺少 Redis 客户端")
	}
	return &SessionRepository{client: client, writer: client, passwordClient: client}
}

// FindByToken 查询访问 Token 对应的用户身份。
func (r *SessionRepository) FindByToken(ctx context.Context, token string) (*user.Session, error) {
	// 1. 查询 Token 对应的会话 JSON
	payload, err := r.client.Get(ctx, authTokenKeyPrefix+token).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, user.ErrSessionNotFound
		}
		return nil, fmt.Errorf("读取用户登录会话: %w", err)
	}
	// 2. 解析并校验必要的用户标识
	var session user.Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, fmt.Errorf("解析用户登录会话: %w", err)
	}
	if session.UserID == 0 {
		return nil, user.ErrSessionNotFound
	}
	return &session, nil
}

// Create 保存 Token 会话并把 Token 加入用户集合。
func (r *SessionRepository) Create(ctx context.Context, token string, session user.Session, ttlSeconds int64) error {
	// 1. 序列化包含用户、设备和登录来源的会话数据
	if r.writer == nil {
		return errors.New("会话仓储不支持写入")
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	// 2. 原子写入 Token 会话和用户多设备 Token 集合
	pipe := r.writer.TxPipeline()
	pipe.Set(ctx, authTokenKeyPrefix+token, payload, time.Duration(ttlSeconds)*time.Second)
	pipe.SAdd(ctx, authUserTokensKeyPrefix+fmt.Sprint(session.UserID), token)
	_, err = pipe.Exec(ctx)
	return err
}

// Delete 删除当前 Token，并从该用户的 Token 集合中移除它。
func (r *SessionRepository) Delete(ctx context.Context, token string, userID uint64) error {
	// 1. 原子删除当前设备 Token 和用户集合成员，不影响其他 Token
	if r.writer == nil {
		return errors.New("会话仓储不支持写入")
	}
	pipe := r.writer.TxPipeline()
	pipe.Del(ctx, authTokenKeyPrefix+token)
	pipe.SRem(ctx, authUserTokensKeyPrefix+fmt.Sprint(userID), token)
	_, err := pipe.Exec(ctx)
	return err
}

// DeleteOtherSessions 原子删除用户除当前 Token 外的所有登录会话。
func (r *SessionRepository) DeleteOtherSessions(ctx context.Context, currentToken string, userID uint64) error {
	if r.writer == nil {
		return errors.New("会话仓储不支持写入")
	}
	// 1. 在同一个 Redis Lua 原子边界内读取用户 Token 集合并删除非当前会话
	const script = `
local tokens = redis.call('SMEMBERS', KEYS[1])
for _, token in ipairs(tokens) do
  if token ~= ARGV[1] then
    redis.call('DEL', ARGV[2] .. token)
    redis.call('SREM', KEYS[1], token)
  end
end
return #tokens
`
	return r.writer.Eval(ctx, script, []string{authUserTokensKeyPrefix + fmt.Sprint(userID)}, currentToken, authTokenKeyPrefix).Err()
}

// CreatePasswordChangeToken 保存十分钟有效的一次性改密凭证。
func (r *SessionRepository) CreatePasswordChangeToken(ctx context.Context, token string, userID uint64, ttl time.Duration) error {
	// 1. 以用户标识为值保存十分钟有效的一次性凭证
	if r.passwordClient == nil {
		return errors.New("改密凭证仓储不支持写入")
	}
	return r.passwordClient.Set(ctx, passwordChangeKeyPrefix+token, userID, ttl).Err()
}

// ConsumePasswordChangeToken 原子消费改密凭证并返回所属用户。
func (r *SessionRepository) ConsumePasswordChangeToken(ctx context.Context, token string) (uint64, time.Duration, error) {
	// 1. Lua 在同一原子边界读取所属用户和剩余 TTL 后删除凭证
	if r.passwordClient == nil {
		return 0, 0, errors.New("改密凭证仓储不支持消费")
	}
	const script = `
local value = redis.call('GET', KEYS[1])
if not value then return nil end
local ttl = redis.call('PTTL', KEYS[1])
redis.call('DEL', KEYS[1])
return {value, ttl}
`
	values, err := r.passwordClient.Eval(ctx, script, []string{passwordChangeKeyPrefix + token}).Slice()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, 0, user.ErrPasswordChangeTokenInvalid
		}
		return 0, 0, err
	}
	if len(values) != 2 {
		return 0, 0, user.ErrPasswordChangeTokenInvalid
	}
	var userID uint64
	if _, err := fmt.Sscan(fmt.Sprint(values[0]), &userID); err != nil || userID == 0 {
		return 0, 0, user.ErrPasswordChangeTokenInvalid
	}
	var ttlMilliseconds int64
	if _, err := fmt.Sscan(fmt.Sprint(values[1]), &ttlMilliseconds); err != nil || ttlMilliseconds <= 0 {
		return 0, 0, user.ErrPasswordChangeTokenInvalid
	}
	return userID, time.Duration(ttlMilliseconds) * time.Millisecond, nil
}

// RestorePasswordChangeToken 在密码事务失败时恢复凭证剩余有效期。
func (r *SessionRepository) RestorePasswordChangeToken(ctx context.Context, token string, userID uint64, ttl time.Duration) error {
	// 1. 只恢复已经验证归属且仍有剩余有效期的凭证
	if r.passwordClient == nil {
		return errors.New("改密凭证仓储不支持恢复")
	}
	if token == "" || userID == 0 || ttl <= 0 {
		return user.ErrPasswordChangeTokenInvalid
	}
	return r.passwordClient.Set(ctx, passwordChangeKeyPrefix+token, userID, ttl).Err()
}
