package repo

import (
	"context"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"xorm.io/xorm"
)

// TestApplyCommentCountEventHandlesDuplicatesAndOutOfOrder 验证重复与乱序事件最终得到正确计数。
func TestApplyCommentCountEventHandlesDuplicatesAndOutOfOrder(t *testing.T) {
	// 1. 删除事件先到时记录较新状态但不扣减零计数
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)
	now := time.Now()
	deleted := article.CommentCountEvent{EventID: "delete-9", EventType: article.CommentDeletedEventType, Version: 2, AggregateID: 9, CommentID: 9, ArticleID: 4, OccurredAt: now}
	if err := repository.ApplyCommentCountEvent(context.Background(), deleted); err != nil {
		t.Fatal(err)
	}
	assertArticleCommentCount(t, engine, 4, 0)

	// 2. 后到的创建事件版本较旧，不能重新增加计数
	created := article.CommentCountEvent{EventID: "create-9", EventType: article.CommentCreatedEventType, Version: 1, AggregateID: 9, CommentID: 9, ArticleID: 4, OccurredAt: now.Add(-time.Second)}
	if err := repository.ApplyCommentCountEvent(context.Background(), created); err != nil {
		t.Fatal(err)
	}
	assertArticleCommentCount(t, engine, 4, 0)

	// 3. 另一评论创建及其重复消息只增加一次，删除及重复也只减少一次
	created = article.CommentCountEvent{EventID: "create-10", EventType: article.CommentCreatedEventType, Version: 1, AggregateID: 10, CommentID: 10, ArticleID: 4, OccurredAt: now}
	if err := repository.ApplyCommentCountEvent(context.Background(), created); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyCommentCountEvent(context.Background(), created); err != nil {
		t.Fatal(err)
	}
	assertArticleCommentCount(t, engine, 4, 1)
	deleted = article.CommentCountEvent{EventID: "delete-10", EventType: article.CommentDeletedEventType, Version: 2, AggregateID: 10, CommentID: 10, ArticleID: 4, OccurredAt: now.Add(time.Second)}
	if err := repository.ApplyCommentCountEvent(context.Background(), deleted); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyCommentCountEvent(context.Background(), deleted); err != nil {
		t.Fatal(err)
	}
	assertArticleCommentCount(t, engine, 4, 0)
}

// TestApplyCommentCountEventRetriesAfterTransactionFailure 验证失败事务回滚后同一消息可正确重试。
func TestApplyCommentCountEventRetriesAfterTransactionFailure(t *testing.T) {
	// 1. 删除文章制造投影更新失败
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)
	if _, err := engine.Exec("DELETE FROM articles WHERE id = 4"); err != nil {
		t.Fatal(err)
	}
	event := article.CommentCountEvent{EventID: "create-retry", EventType: article.CommentCreatedEventType, Version: 1, AggregateID: 20, CommentID: 20, ArticleID: 4, OccurredAt: time.Now()}
	if err := repository.ApplyCommentCountEvent(context.Background(), event); err == nil {
		t.Fatal("missing article did not fail")
	}
	var inboxCount int64
	if _, err := engine.SQL("SELECT COUNT(*) FROM article_comment_event_inbox WHERE event_id = ?", event.EventID).Get(&inboxCount); err != nil || inboxCount != 0 {
		t.Fatalf("inbox count = %d, error=%v", inboxCount, err)
	}

	// 2. 恢复文章后重试同一事件，Inbox 和评论数各生效一次
	if _, err := engine.Exec("INSERT INTO articles (id, author_id, title, content, tags, status, created_time, updated_time) VALUES (?, ?, ?, ?, '', ?, ?, ?)", 4, 7, "已恢复", "正文", article.StatusPublished, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := repository.ApplyCommentCountEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.SQL("SELECT COUNT(*) FROM article_comment_event_inbox WHERE event_id = ?", event.EventID).Get(&inboxCount); err != nil || inboxCount != 1 {
		t.Fatalf("retried inbox count = %d, error=%v", inboxCount, err)
	}
	assertArticleCommentCount(t, engine, 4, 1)
}

// assertArticleCommentCount 校验文章评论数投影。
func assertArticleCommentCount(t *testing.T, engine *xorm.Engine, articleID uint64, want int64) {
	// 1. 查询文章权威评论数
	t.Helper()
	var count int64
	found, err := engine.SQL("SELECT comment_count FROM articles WHERE id = ?", articleID).Get(&count)
	if err != nil || !found || count != want {
		t.Fatalf("comment_count = %d, want %d, found=%v, error=%v", count, want, found, err)
	}
}
