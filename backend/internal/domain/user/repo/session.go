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
)

type sessionGetter interface {
	// Get 读取指定会话 Key。
	Get(context.Context, string) *redis.StringCmd
}

// sessionWriter 定义会话创建和删除所需的 Redis 原子流水线能力。
type sessionWriter interface {
	sessionGetter
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
	SAdd(context.Context, string, ...interface{}) *redis.IntCmd
	SRem(context.Context, string, ...interface{}) *redis.IntCmd
	TxPipeline() redis.Pipeliner
}

// SessionRepository 使用 Redis 保存用户登录会话。
type SessionRepository struct {
	client sessionGetter // client 提供登录会话查询能力。
	writer sessionWriter // writer 提供登录会话原子写入能力。
}

// NewSessionRepository 创建用户会话 Redis 仓储。
func NewSessionRepository(client clients.RedisClient) *SessionRepository {
	// 1. 启动阶段拒绝缺少 Redis 客户端的会话仓储
	if client == nil {
		panic("用户会话仓储缺少 Redis 客户端")
	}
	return &SessionRepository{client: client, writer: client}
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
