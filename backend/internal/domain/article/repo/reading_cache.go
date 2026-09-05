package repo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"github.com/redis/go-redis/v9"
)

const (
	viewEventKeyPrefix = "article:view:event:" // viewEventKeyPrefix 是浏览事件幂等状态前缀。
	hotRankKey         = "article:hot-rank"    // hotRankKey 是文章热榜 Sorted Set。
	viewProcessing     = "processing"          // viewProcessing 表示事件正在处理。
	viewCompleted      = "completed"           // viewCompleted 表示事件已处理完成。
)

var beginViewEventScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if value == "completed" then return 3 end
if value == "processing" then return 2 end
redis.call("SET", KEYS[1], "processing", "PX", ARGV[1])
return 1
`)

var releaseViewEventScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == "processing" then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

// ReadingCache 使用 Redis 实现浏览事件幂等状态和文章热榜投影。
type ReadingCache struct {
	client clients.RedisClient // client 提供 Redis 字符串和 Sorted Set 能力。
}

// NewReadingCache 创建文章阅读 Redis 适配器。
func NewReadingCache(client clients.RedisClient) *ReadingCache {
	// 1. 启动阶段拒绝缺少 Redis 客户端
	if client == nil {
		panic("文章阅读缓存缺少 Redis 客户端")
	}
	return &ReadingCache{client: client}
}

// Begin 原子读取或占用浏览事件处理状态。
func (r *ReadingCache) Begin(ctx context.Context, eventID string, ttl time.Duration) (article.ViewEventState, error) {
	// 1. Lua 原子区分首次处理、处理中和已完成状态
	value, err := beginViewEventScript.Run(ctx, r.client, []string{viewEventKey(eventID)}, ttl.Milliseconds()).Int64()
	return article.ViewEventState(value), err
}

// Complete 将浏览事件标记为处理完成。
func (r *ReadingCache) Complete(ctx context.Context, eventID string, ttl time.Duration) error {
	// 1. 完成标记保留较长时间，覆盖 Kafka 延迟重放窗口
	return r.client.Set(ctx, viewEventKey(eventID), viewCompleted, ttl).Err()
}

// Release 在权威数据库写入失败时释放处理权。
func (r *ReadingCache) Release(ctx context.Context, eventID string) error {
	// 1. 仅删除仍处于 processing 的状态，避免覆盖已完成标记
	return releaseViewEventScript.Run(ctx, r.client, []string{viewEventKey(eventID)}).Err()
}

// Top 查询热度最高的文章标识和分值。
func (r *ReadingCache) Top(ctx context.Context, limit int64) ([]article.RankEntry, error) {
	// 1. Redis 按分值倒序返回前 N 项
	values, err := r.client.ZRevRangeWithScores(ctx, hotRankKey, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]article.RankEntry, 0, len(values))
	for _, value := range values {
		articleID, err := strconv.ParseUint(fmt.Sprint(value.Member), 10, 64)
		if err != nil || articleID == 0 {
			continue
		}
		entries = append(entries, article.RankEntry{ArticleID: articleID, Score: value.Score})
	}
	return entries, nil
}

// SetScore 使用权威统计覆盖单篇文章热度。
func (r *ReadingCache) SetScore(ctx context.Context, articleID uint64, score int64) error {
	// 1. 覆盖而非累加，保证重复事件可修复缓存漂移
	return r.client.ZAdd(ctx, hotRankKey, redis.Z{Score: float64(score), Member: strconv.FormatUint(articleID, 10)}).Err()
}

// Replace 使用 MySQL 权威统计原子覆盖热榜。
func (r *ReadingCache) Replace(ctx context.Context, metrics []*article.HotMetric) error {
	// 1. MULTI/EXEC 保证查询只看到替换前或替换后的完整集合
	_, err := r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, hotRankKey)
		if len(metrics) == 0 {
			return nil
		}
		members := make([]redis.Z, 0, len(metrics))
		for _, metric := range metrics {
			members = append(members, redis.Z{Score: float64(metric.Score()), Member: strconv.FormatUint(metric.ArticleID, 10)})
		}
		pipe.ZAdd(ctx, hotRankKey, members...)
		return nil
	})
	return err
}

// viewEventKey 返回浏览事件幂等状态键。
func viewEventKey(eventID string) string {
	// 1. 使用稳定事件标识隔离每次文章访问
	return viewEventKeyPrefix + eventID
}
