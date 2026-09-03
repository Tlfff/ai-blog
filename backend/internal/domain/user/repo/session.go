package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"github.com/redis/go-redis/v9"
)

const authTokenKeyPrefix = "auth:token:"

// sessionGetter 是会话仓储实际使用的最小 Redis 查询接缝。
type sessionGetter interface {
	// Get 读取指定 Redis Key 的字符串值。
	Get(ctx context.Context, key string) *redis.StringCmd
}

// SessionRepository 使用 Redis 查询用户登录会话。
type SessionRepository struct {
	client sessionGetter // client 是用户会话使用的最小 Redis 查询客户端。
}

// NewSessionRepository 创建用户会话 Redis 仓储。
func NewSessionRepository(client clients.RedisClient) user.SessionRepository {
	// 1. 启动阶段拒绝缺少 Redis 客户端的会话仓储
	if client == nil {
		panic("用户会话仓储缺少 Redis 客户端")
	}
	return &SessionRepository{client: client}
}

// FindByToken 查询访问 Token 对应的用户身份。
func (r *SessionRepository) FindByToken(ctx context.Context, token string) (*user.Session, error) {
	// 1. 从约定的 Token Key 读取会话 JSON
	payload, err := r.client.Get(ctx, authTokenKeyPrefix+token).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, user.ErrSessionNotFound
		}
		return nil, fmt.Errorf("读取用户登录会话: %w", err)
	}

	// 2. 解析并校验会话中的必要身份字段
	var session user.Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, fmt.Errorf("解析用户登录会话: %w", err)
	}
	if session.UserID == 0 {
		return nil, user.ErrSessionNotFound
	}
	return &session, nil
}
