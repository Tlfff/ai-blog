package repo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/entity"
	"github.com/redis/go-redis/v9"
)

// fakeRedisLikeClient 使用内存集合实现点赞缓存测试命令。
type fakeRedisLikeClient struct {
	sets map[string]map[string]struct{} // sets 保存 Redis Set 成员。
	err  error                          // err 是所有命令的预设错误。
}

// SIsMember 查询测试集合成员。
func (f *fakeRedisLikeClient) SIsMember(_ context.Context, key string, member interface{}) *redis.BoolCmd {
	// 1. 返回内存集合查询结果
	_, found := f.sets[key][fmt.Sprint(member)]
	return redis.NewBoolResult(found, f.err)
}

// SAdd 添加测试集合成员。
func (f *fakeRedisLikeClient) SAdd(_ context.Context, key string, members ...interface{}) *redis.IntCmd {
	// 1. 将全部成员写入内存集合
	if f.err != nil {
		return redis.NewIntResult(0, f.err)
	}
	if f.sets[key] == nil {
		f.sets[key] = make(map[string]struct{})
	}
	for _, member := range members {
		f.sets[key][fmt.Sprint(member)] = struct{}{}
	}
	return redis.NewIntResult(int64(len(members)), nil)
}

// SRem 删除测试集合成员。
func (f *fakeRedisLikeClient) SRem(_ context.Context, key string, members ...interface{}) *redis.IntCmd {
	// 1. 从内存集合删除全部成员
	if f.err != nil {
		return redis.NewIntResult(0, f.err)
	}
	for _, member := range members {
		delete(f.sets[key], fmt.Sprint(member))
	}
	return redis.NewIntResult(int64(len(members)), nil)
}

// Scan 返回符合点赞键前缀的全部测试键。
func (f *fakeRedisLikeClient) Scan(_ context.Context, cursor uint64, match string, _ int64) *redis.ScanCmd {
	// 1. 单页返回与前缀匹配的键
	if cursor != 0 {
		return redis.NewScanCmdResult(nil, 0, f.err)
	}
	prefix := strings.TrimSuffix(match, "*")
	keys := make([]string, 0)
	for key := range f.sets {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return redis.NewScanCmdResult(keys, 0, f.err)
}

// Del 删除测试键。
func (f *fakeRedisLikeClient) Del(_ context.Context, keys ...string) *redis.IntCmd {
	// 1. 删除全部指定集合
	if f.err != nil {
		return redis.NewIntResult(0, f.err)
	}
	for _, key := range keys {
		delete(f.sets, key)
	}
	return redis.NewIntResult(int64(len(keys)), nil)
}

// TestCacheRebuildsSetsAndRemovesStaleMembers 验证 MySQL 事实可完整重建 Redis 集合。
func TestCacheRebuildsSetsAndRemovesStaleMembers(t *testing.T) {
	// 1. 预置脏集合并使用当前事实完整替换
	client := &fakeRedisLikeClient{sets: map[string]map[string]struct{}{
		articleLikeKey(9):  {"99": {}},
		articleLikeKey(10): {"88": {}},
	}}
	cache := &Cache{client: client}
	facts := []*entity.ArticleLike{{UserID: 7, ArticleID: 9}, {UserID: 8, ArticleID: 9}, {UserID: 5, ArticleID: 11}}
	if err := cache.ReplaceArticleLikes(context.Background(), facts); err != nil {
		t.Fatal(err)
	}

	// 2. 旧成员和无事实文章集合被删除，新事实成员全部存在
	if len(client.sets[articleLikeKey(9)]) != 2 {
		t.Fatalf("article 9 members = %#v", client.sets[articleLikeKey(9)])
	}
	if _, found := client.sets[articleLikeKey(9)]["99"]; found {
		t.Fatal("stale member remains")
	}
	if _, found := client.sets[articleLikeKey(10)]; found {
		t.Fatal("stale article set remains")
	}
	if _, found := client.sets[articleLikeKey(11)]["5"]; !found {
		t.Fatal("rebuilt member missing")
	}
}
