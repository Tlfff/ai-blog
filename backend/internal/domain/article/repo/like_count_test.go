package repo

import (
	"context"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"xorm.io/xorm"
)

// TestApplyLikeCountEventHandlesDuplicatesAndOutOfOrder 验证重复与乱序事件最终得到正确非负计数。
func TestApplyLikeCountEventHandlesDuplicatesAndOutOfOrder(t *testing.T) {
	// 1. 取消事件先到时记录较新状态但不把零计数减为负数
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)
	now := time.Now()
	unliked := article.LikeCountEvent{EventID: "unlike-9-v2", EventType: article.ArticleUnlikedEventType, Version: 2, AggregateID: 9, LikeID: 9, ArticleID: 4, UserID: 7, OccurredAt: now}
	if err := repository.ApplyLikeCountEvent(context.Background(), unliked); err != nil {
		t.Fatal(err)
	}
	assertArticleLikeCount(t, engine, 4, 0)

	// 2. 后到的旧点赞事件不能重新增加计数，新版本点赞及重复消息只增加一次
	liked := article.LikeCountEvent{EventID: "like-9-v1", EventType: article.ArticleLikedEventType, Version: 1, AggregateID: 9, LikeID: 9, ArticleID: 4, UserID: 7, OccurredAt: now.Add(-time.Second)}
	if err := repository.ApplyLikeCountEvent(context.Background(), liked); err != nil {
		t.Fatal(err)
	}
	assertArticleLikeCount(t, engine, 4, 0)
	liked = article.LikeCountEvent{EventID: "like-9-v3", EventType: article.ArticleLikedEventType, Version: 3, AggregateID: 9, LikeID: 9, ArticleID: 4, UserID: 7, OccurredAt: now.Add(time.Second)}
	if err := repository.ApplyLikeCountEvent(context.Background(), liked); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyLikeCountEvent(context.Background(), liked); err != nil {
		t.Fatal(err)
	}
	assertArticleLikeCount(t, engine, 4, 1)

	// 3. 新版本取消及重复消息只扣减一次，计数保持非负
	unliked = article.LikeCountEvent{EventID: "unlike-9-v4", EventType: article.ArticleUnlikedEventType, Version: 4, AggregateID: 9, LikeID: 9, ArticleID: 4, UserID: 7, OccurredAt: now.Add(2 * time.Second)}
	if err := repository.ApplyLikeCountEvent(context.Background(), unliked); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyLikeCountEvent(context.Background(), unliked); err != nil {
		t.Fatal(err)
	}
	assertArticleLikeCount(t, engine, 4, 0)
}

// TestApplyLikeCountEventRetriesAfterTransactionFailure 验证失败事务回滚后同一消息可正确重试。
func TestApplyLikeCountEventRetriesAfterTransactionFailure(t *testing.T) {
	// 1. 删除文章制造投影更新失败，Inbox 不得提前提交
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)
	if _, err := engine.Exec("DELETE FROM articles WHERE id = 4"); err != nil {
		t.Fatal(err)
	}
	event := article.LikeCountEvent{EventID: "like-retry", EventType: article.ArticleLikedEventType, Version: 1, AggregateID: 20, LikeID: 20, ArticleID: 4, UserID: 7, OccurredAt: time.Now()}
	if err := repository.ApplyLikeCountEvent(context.Background(), event); err == nil {
		t.Fatal("missing article did not fail")
	}
	assertInboxCount(t, engine, event.EventID, 0)

	// 2. 恢复文章后重试同一事件，Inbox 和点赞数各生效一次
	if _, err := engine.Exec("INSERT INTO articles (id, author_id, title, content, tags, status, created_time, updated_time) VALUES (?, ?, ?, ?, '', ?, ?, ?)", 4, 7, "已恢复", "正文", article.StatusPublished, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyLikeCountEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	assertInboxCount(t, engine, event.EventID, 1)
	assertArticleLikeCount(t, engine, 4, 1)
}

// assertArticleLikeCount 校验文章点赞数投影。
func assertArticleLikeCount(t *testing.T, engine *xorm.Engine, articleID uint64, want int64) {
	// 1. 查询文章上下文拥有的点赞数
	t.Helper()
	var count int64
	found, err := engine.SQL("SELECT like_count FROM articles WHERE id = ?", articleID).Get(&count)
	if err != nil || !found || count != want {
		t.Fatalf("like_count=%d want=%d found=%v error=%v", count, want, found, err)
	}
}

// assertInboxCount 校验点赞事件 Inbox 事务结果。
func assertInboxCount(t *testing.T, engine *xorm.Engine, eventID string, want int64) {
	// 1. 查询指定事件的 Inbox 记录数
	t.Helper()
	var count int64
	if _, err := engine.SQL("SELECT COUNT(*) FROM article_like_event_inbox WHERE event_id = ?", eventID).Get(&count); err != nil || count != want {
		t.Fatalf("inbox count=%d want=%d error=%v", count, want, err)
	}
}
