package repo

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"context"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"testing"
	"time"
	"xorm.io/xorm"
)

// TestCommentLikeCountProjectionHandlesOrderAndRebuild 验证乱序幂等、非负和事实重建。
func TestCommentLikeCountProjectionHandlesOrderAndRebuild(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	for _, q := range []string{
		`CREATE TABLE comments (id INTEGER PRIMARY KEY, like_count INTEGER NOT NULL DEFAULT 0, status INTEGER NOT NULL)`,
		`CREATE TABLE comment_like_projection_lock (id INTEGER PRIMARY KEY)`,
		`INSERT INTO comment_like_projection_lock(id) VALUES(1)`,
		`CREATE TABLE comment_like_event_inbox (event_id TEXT PRIMARY KEY, like_id INTEGER, comment_id INTEGER, processed_time DATETIME)`,
		`CREATE TABLE comment_like_projection (like_id INTEGER PRIMARY KEY, comment_id INTEGER, user_id INTEGER, version INTEGER, active INTEGER, last_event_id TEXT, updated_time DATETIME, UNIQUE(user_id,comment_id))`,
		`CREATE TABLE comment_likes (id INTEGER PRIMARY KEY, user_id INTEGER, comment_id INTEGER, status INTEGER)`,
		`CREATE TABLE comment_like_event_outbox (event_id TEXT PRIMARY KEY, aggregate_id INTEGER, version INTEGER)`,
		`INSERT INTO comments(id,like_count,status) VALUES(9,0,1)`,
	} {
		if _, err := engine.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	r := &Repository{client: engine, transaction: engine}
	now := time.Now()
	unlike := comment.LikeCountEvent{EventID: "u2", EventType: comment.CommentUnlikedEventType, Version: 2, AggregateID: 1, LikeID: 1, CommentID: 9, UserID: 7, OccurredAt: now}
	likeEvent := comment.LikeCountEvent{EventID: "l1", EventType: comment.CommentLikedEventType, Version: 1, AggregateID: 1, LikeID: 1, CommentID: 9, UserID: 7, OccurredAt: now}
	if err := r.ApplyLikeCountEvent(context.Background(), unlike); err != nil {
		t.Fatal(err)
	}
	if err := r.ApplyLikeCountEvent(context.Background(), likeEvent); err != nil {
		t.Fatal(err)
	}
	assertCommentCount(t, engine, 0)
	_, _ = engine.Exec(`INSERT INTO comment_likes VALUES(1,7,9,1)`)
	_, _ = engine.Exec(`INSERT INTO comment_like_event_outbox VALUES('l3',1,3)`)
	if err := r.RebuildLikeCountProjection(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertCommentCount(t, engine, 1)
}

// assertCommentCount 校验评论点赞数投影。
func assertCommentCount(t *testing.T, e *xorm.Engine, w int64) {
	t.Helper()
	var n int64
	_, err := e.SQL("SELECT like_count FROM comments WHERE id=9").Get(&n)
	if err != nil || n != w {
		t.Fatalf("count=%d want=%d err=%v", n, w, err)
	}
}
