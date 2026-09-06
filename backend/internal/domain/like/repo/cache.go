package repo

import (
	"context"
	"strconv"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/entity"
	"github.com/redis/go-redis/v9"
)

const articleLikeKeyPrefix = "article:likes:"

// redisLikeClient 是点赞集合使用的最小 Redis 命令集合。
type redisLikeClient interface {
	SIsMember(context.Context, string, interface{}) *redis.BoolCmd
	SAdd(context.Context, string, ...interface{}) *redis.IntCmd
	SRem(context.Context, string, ...interface{}) *redis.IntCmd
	Scan(context.Context, uint64, string, int64) *redis.ScanCmd
	Del(context.Context, ...string) *redis.IntCmd
}

// Cache 使用 Redis Set 实现可重建的文章点赞集合。
type Cache struct {
	client redisLikeClient // client 提供 Redis Set 与扫描能力。
}

// NewCache 创建文章点赞 Redis 缓存。
func NewCache(client clients.RedisClient) *Cache {
	// 1. 启动阶段拒绝缺少 Redis 客户端
	if client == nil {
		panic("文章点赞缓存缺少 Redis 客户端")
	}
	return &Cache{client: client}
}

// IsArticleLiked 查询 Redis 集合中是否存在点赞用户。
func (c *Cache) IsArticleLiked(ctx context.Context, articleID, userID uint64) (bool, error) {
	// 1. Set 成员使用十进制用户标识保持跨语言兼容
	return c.client.SIsMember(ctx, articleLikeKey(articleID), strconv.FormatUint(userID, 10)).Result()
}

// StoreArticleLike 使用最终事实覆盖单个用户的缓存状态。
func (c *Cache) StoreArticleLike(ctx context.Context, articleID, userID uint64, liked bool) error {
	// 1. Redis 只保存可丢失投影，命令失败由调用方忽略并等待重建
	member := strconv.FormatUint(userID, 10)
	if liked {
		return c.client.SAdd(ctx, articleLikeKey(articleID), member).Err()
	}
	return c.client.SRem(ctx, articleLikeKey(articleID), member).Err()
}

// ReplaceArticleLikes 使用 MySQL 当前事实重建全部点赞集合。
func (c *Cache) ReplaceArticleLikes(ctx context.Context, facts []*entity.ArticleLike) error {
	// 1. 先删除全部旧文章点赞集合，避免取消点赞后的脏成员残留
	var cursor uint64
	keysToDelete := make([]string, 0)
	for {
		keys, next, err := c.client.Scan(ctx, cursor, articleLikeKeyPrefix+"*", 500).Result()
		if err != nil {
			return err
		}
		keysToDelete = append(keysToDelete, keys...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if len(keysToDelete) > 0 {
		if err := c.client.Del(ctx, keysToDelete...).Err(); err != nil {
			return err
		}
	}

	// 2. 按文章分组写入全部有效点赞用户
	members := make(map[uint64][]interface{})
	for _, fact := range facts {
		if fact == nil || fact.ArticleID == 0 || fact.UserID == 0 {
			continue
		}
		members[fact.ArticleID] = append(members[fact.ArticleID], strconv.FormatUint(fact.UserID, 10))
	}
	for articleID, users := range members {
		if err := c.client.SAdd(ctx, articleLikeKey(articleID), users...).Err(); err != nil {
			return err
		}
	}
	return nil
}

// articleLikeKey 返回单篇文章点赞用户集合键。
func articleLikeKey(articleID uint64) string {
	// 1. 每篇文章使用独立 Set 支持直接成员查询和重建
	return articleLikeKeyPrefix + strconv.FormatUint(articleID, 10)
}
