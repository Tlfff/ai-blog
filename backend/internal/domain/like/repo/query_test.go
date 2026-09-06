package repo

import (
	"context"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
)

// TestQueryRepositoryUsesMySQLFactAndRepairsRedis 验证陈旧或失败缓存不能覆盖点赞事实。
func TestQueryRepositoryUsesMySQLFactAndRepairsRedis(t *testing.T) {
	// 1. Redis 陈旧正缓存存在但 MySQL 已取消时返回未点赞并清除成员
	_, engine := newLikeTestRepository(t)
	defer engine.Close()
	if _, err := engine.Exec("INSERT INTO article_likes (id, user_id, article_id, status, created_time, updated_time) VALUES (1, 7, 9, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", like.StatusUnliked); err != nil {
		t.Fatal(err)
	}
	client := &fakeRedisLikeClient{sets: map[string]map[string]struct{}{articleLikeKey(9): {"7": {}}}}
	query := &QueryRepository{client: engine, cache: &Cache{client: client}}
	liked, err := query.IsArticleLiked(context.Background(), 7, 9)
	if err != nil || liked {
		t.Fatalf("liked=%v error=%v", liked, err)
	}
	if _, found := client.sets[articleLikeKey(9)]["7"]; found {
		t.Fatal("stale positive cache was not repaired")
	}

	// 2. MySQL 点赞事实存在时返回已点赞并修复空缓存
	if _, err := engine.Exec("UPDATE article_likes SET status = ? WHERE id = 1", like.StatusLiked); err != nil {
		t.Fatal(err)
	}
	liked, err = query.IsArticleLiked(context.Background(), 7, 9)
	if err != nil || !liked {
		t.Fatalf("liked=%v error=%v", liked, err)
	}
	if _, found := client.sets[articleLikeKey(9)]["7"]; !found {
		t.Fatal("positive cache was not repaired")
	}
}
