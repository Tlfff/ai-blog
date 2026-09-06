package repo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

// TestChangeArticleLikeIsIdempotentAndVersionsTransitions 验证重复操作不重复改变事实或生成事件。
func TestChangeArticleLikeIsIdempotentAndVersionsTransitions(t *testing.T) {
	// 1. 首次点赞生成版本1，重复点赞不新增事件
	repository, engine := newLikeTestRepository(t)
	defer engine.Close()
	now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC)
	changed, err := repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusLiked, now)
	if err != nil || !changed {
		t.Fatalf("first like changed=%v error=%v", changed, err)
	}
	changed, err = repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusLiked, now.Add(time.Second))
	if err != nil || changed {
		t.Fatalf("duplicate like changed=%v error=%v", changed, err)
	}
	assertLikeFactAndEvents(t, engine, like.StatusLiked, 1, 1)

	// 2. 首次取消生成版本2，重复取消不新增事件
	changed, err = repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusUnliked, now.Add(2*time.Second))
	if err != nil || !changed {
		t.Fatalf("first cancel changed=%v error=%v", changed, err)
	}
	changed, err = repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusUnliked, now.Add(3*time.Second))
	if err != nil || changed {
		t.Fatalf("duplicate cancel changed=%v error=%v", changed, err)
	}
	assertLikeFactAndEvents(t, engine, like.StatusUnliked, 2, 2)

	// 3. 再次点赞生成版本3，证明同一关系跨多次切换仍单调
	changed, err = repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusLiked, now.Add(4*time.Second))
	if err != nil || !changed {
		t.Fatalf("second like changed=%v error=%v", changed, err)
	}
	assertLikeFactAndEvents(t, engine, like.StatusLiked, 3, 3)
}

// TestCancelMissingLikeIsSuccessfulNoOp 验证从未点赞的取消不创建事实或事件。
func TestCancelMissingLikeIsSuccessfulNoOp(t *testing.T) {
	// 1. 空关系取消返回未变化且数据库保持为空
	repository, engine := newLikeTestRepository(t)
	defer engine.Close()
	changed, err := repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusUnliked, time.Now())
	if err != nil || changed {
		t.Fatalf("cancel missing changed=%v error=%v", changed, err)
	}
	assertTableCount(t, engine, "article_likes", 0)
	assertTableCount(t, engine, "article_like_event_outbox", 0)
}

// TestChangeArticleLikeRollsBackWhenOutboxInsertFails 验证点赞事实与 Outbox 同事务提交。
func TestChangeArticleLikeRollsBackWhenOutboxInsertFails(t *testing.T) {
	// 1. 删除 Outbox 表后首次点赞失败且不留下点赞关系
	repository, engine := newLikeTestRepository(t)
	defer engine.Close()
	if _, err := engine.Exec("DROP TABLE article_like_event_outbox"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusLiked, time.Now()); err == nil {
		t.Fatal("outbox insert failure did not fail like")
	}
	assertTableCount(t, engine, "article_likes", 0)
}

// TestCancelRollsBackWhenOutboxInsertFails 验证取消事件失败会恢复原点赞事实。
func TestCancelRollsBackWhenOutboxInsertFails(t *testing.T) {
	// 1. 准备已点赞关系并删除 Outbox 表制造事务失败
	repository, engine := newLikeTestRepository(t)
	defer engine.Close()
	if _, err := engine.Exec("INSERT INTO article_likes (id, user_id, article_id, status, created_time, updated_time) VALUES (1, 7, 9, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec("DROP TABLE article_like_event_outbox"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusUnliked, time.Now()); err == nil {
		t.Fatal("outbox insert failure did not fail cancel")
	}
	var status int
	if _, err := engine.SQL("SELECT status FROM article_likes WHERE id = 1").Get(&status); err != nil || status != int(like.StatusLiked) {
		t.Fatalf("status=%d error=%v", status, err)
	}
}

// TestListActiveArticleLikesUsesMySQLFacts 验证缓存重建只读取当前有效点赞关系。
func TestListActiveArticleLikesUsesMySQLFacts(t *testing.T) {
	// 1. 混合点赞与取消事实后只返回状态1记录
	repository, engine := newLikeTestRepository(t)
	defer engine.Close()
	if _, err := engine.Exec("INSERT INTO article_likes (id, user_id, article_id, status, created_time, updated_time) VALUES (1, 7, 9, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), (2, 8, 9, 2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)"); err != nil {
		t.Fatal(err)
	}
	facts, err := repository.ListActiveArticleLikes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 || facts[0].UserID != 7 || facts[0].ArticleID != 9 {
		t.Fatalf("facts = %#v", facts)
	}
}

// assertLikeFactAndEvents 校验点赞状态、事件数量和最大版本。
//
// 参数说明：
//   - t：当前测试上下文。
//   - engine：隔离数据库引擎。
//   - wantStatus：期望点赞事实状态。
//   - wantEvents：期望 Outbox 事件数量。
//   - wantVersion：期望最大聚合版本。
func assertLikeFactAndEvents(t *testing.T, engine *xorm.Engine, wantStatus int8, wantEvents, wantVersion int64) {
	// 1. 查询事实状态和 Outbox 聚合版本
	t.Helper()
	var status int
	if _, err := engine.SQL("SELECT status FROM article_likes WHERE user_id = 7 AND article_id = 9").Get(&status); err != nil || status != int(wantStatus) {
		t.Fatalf("status=%d want=%d error=%v", status, wantStatus, err)
	}
	var result struct {
		Count   int64 `xorm:"'count'"`   // Count 是 Outbox 事件数。
		Version int64 `xorm:"'version'"` // Version 是最大聚合版本。
	}
	found, err := engine.SQL("SELECT COUNT(*) AS count, COALESCE(MAX(version), 0) AS version FROM article_like_event_outbox WHERE aggregate_id = 1").Get(&result)
	if err != nil || !found || result.Count != wantEvents || result.Version != wantVersion {
		t.Fatalf("outbox=%#v found=%v error=%v", result, found, err)
	}
}

// assertTableCount 校验测试表记录数量。
func assertTableCount(t *testing.T, engine *xorm.Engine, table string, want int64) {
	// 1. 查询完整表记录数量
	t.Helper()
	count, err := engine.Table(table).Count()
	if err != nil || count != want {
		t.Fatalf("%s count=%d want=%d error=%v", table, count, want, err)
	}
}

// newLikeTestRepository 创建点赞仓储使用的隔离内存数据库。
func newLikeTestRepository(t *testing.T) (*Repository, *xorm.Engine) {
	// 1. 使用测试名称隔离共享内存数据库并建立唯一关系约束
	t.Helper()
	engine, err := xorm.NewEngine("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE article_likes (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, article_id INTEGER NOT NULL, status INTEGER NOT NULL, created_time DATETIME NOT NULL, updated_time DATETIME NOT NULL, UNIQUE(user_id, article_id))`,
		`CREATE TABLE article_like_event_outbox (event_id TEXT PRIMARY KEY, aggregate_id INTEGER NOT NULL, event_type TEXT NOT NULL, version INTEGER NOT NULL, occurred_at DATETIME NOT NULL, payload TEXT NOT NULL, status INTEGER NOT NULL DEFAULT 0, attempts INTEGER NOT NULL DEFAULT 0, next_attempt_time DATETIME NOT NULL, published_time DATETIME NULL, last_error TEXT NOT NULL DEFAULT '', created_time DATETIME NOT NULL, updated_time DATETIME NOT NULL, UNIQUE(aggregate_id, version))`,
	} {
		if _, err := engine.Exec(statement); err != nil {
			engine.Close()
			t.Fatal(err)
		}
	}
	nextID := 0
	repository := &Repository{client: engine, transaction: engine, now: time.Now, newEventID: func() string {
		nextID++
		return fmt.Sprintf("like-event-%d", nextID)
	}}
	return repository, engine
}
