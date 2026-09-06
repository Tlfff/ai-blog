package repo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo/po"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

// TestCreateReplyUpdatesRootCountInTransaction 模拟当前评论测试所需的依赖行为。
func TestCreateReplyUpdatesRootCountInTransaction(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	created := time.Now().Truncate(time.Second)
	for i := 0; i < 2; i++ {
		reply := &entity.Comment{ArticleID: 1, UserID: uint64(i + 2), RootID: 1, ReplyToUserID: 7, Content: fmt.Sprintf("回复%d", i), Status: comment.StatusNormal, CreatedTime: created, UpdatedTime: created}
		if err := repository.Create(context.Background(), reply); err != nil {
			t.Fatalf("Create reply: %v", err)
		}
	}
	root := new(po.Comment)
	found, err := engine.Table("comments").Where("id = ?", 1).Get(root)
	if err != nil || !found {
		t.Fatalf("query root: found=%v error=%v", found, err)
	}
	if root.CommentCount != 2 {
		t.Fatalf("root comment_count = %d, want 2", root.CommentCount)
	}
	var count int64
	count, err = engine.Table("comments").Where("root_id = ? AND status = ?", 1, comment.StatusNormal).Count()
	if err != nil || count != 2 {
		t.Fatalf("reply count = %d, error=%v", count, err)
	}
}

// TestCreateReplyRejectsNestedReplyAndDeletedRoot 模拟当前评论测试所需的依赖行为。
func TestCreateReplyRejectsNestedReplyAndDeletedRoot(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	deleted := &entity.Comment{ArticleID: 1, UserID: 3, RootID: 1, Content: "不应写入", Status: comment.StatusNormal, CreatedTime: time.Now(), UpdatedTime: time.Now()}
	if err := repository.Create(context.Background(), deleted); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec("UPDATE comments SET status = ? WHERE id = ?", comment.StatusDeleted, 1); err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(context.Background(), &entity.Comment{ArticleID: 1, UserID: 4, RootID: 1, Content: "删除根回复", Status: comment.StatusNormal, CreatedTime: time.Now(), UpdatedTime: time.Now()}); err != comment.ErrRootDeleted {
		t.Fatalf("error = %v", err)
	}
	if err := repository.Create(context.Background(), &entity.Comment{ArticleID: 1, UserID: 4, RootID: 2, Content: "嵌套回复", Status: comment.StatusNormal, CreatedTime: time.Now(), UpdatedTime: time.Now()}); err != comment.ErrRootDeleted && err != comment.ErrRootNotFound {
		t.Fatalf("nested error = %v", err)
	}
}

// TestListRootsAndRepliesSupportsCursorAndAuthorFilter 模拟当前评论测试所需的依赖行为。
func TestListRootsAndRepliesSupportsCursorAndAuthorFilter(t *testing.T) {
	// 1. 执行当前测试依赖操作并记录断言数据
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	for i := 0; i < 3; i++ {
		root := &entity.Comment{ArticleID: 1, UserID: uint64(i + 2), Content: fmt.Sprintf("主评论%d", i), Status: comment.StatusNormal, CreatedTime: time.Now(), UpdatedTime: time.Now()}
		if err := repository.Create(context.Background(), root); err != nil {
			t.Fatal(err)
		}
	}
	roots, err := repository.ListRoots(context.Background(), comment.RootListQuery{ArticleID: 1, AuthorID: 3, PageQuery: comment.PageQuery{Page: 1, PageSize: 10}})
	if err != nil || roots.Total != 1 || len(roots.Items) != 1 {
		t.Fatalf("roots = %#v, error=%v", roots, err)
	}
	page, err := repository.ListRoots(context.Background(), comment.RootListQuery{ArticleID: 1, PageQuery: comment.PageQuery{Page: 1, PageSize: 2}})
	if err != nil || len(page.Items) != 2 || page.LastID == 0 {
		t.Fatalf("page = %#v, error=%v", page, err)
	}
	if page.Items[0].Comment.LikeCount != 4 {
		t.Fatalf("like_count = %d, want 4", page.Items[0].Comment.LikeCount)
	}
	cursor, err := repository.ListRoots(context.Background(), comment.RootListQuery{ArticleID: 1, PageQuery: comment.PageQuery{LastID: page.LastID, Page: 1, PageSize: 2}})
	if err != nil || len(cursor.Items) != 2 {
		t.Fatalf("cursor = %#v, error=%v", cursor, err)
	}
}

// newCommentTestRepository 模拟当前评论测试所需的依赖行为。
func newCommentTestRepository(t *testing.T) (*Repository, *xorm.Engine) {
	// 1. 执行当前测试依赖操作并记录断言数据
	t.Helper()
	engine, err := xorm.NewEngine("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	engine.SetMaxOpenConns(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE articles (id INTEGER PRIMARY KEY, status INTEGER NOT NULL, comment_count INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO articles (id, status, comment_count) VALUES (1, 3, 0)`,
		`CREATE TABLE comments (id INTEGER PRIMARY KEY AUTOINCREMENT, article_id INTEGER NOT NULL, user_id INTEGER NOT NULL, reply_to_user_id INTEGER NOT NULL DEFAULT 0, content TEXT NOT NULL, root_id INTEGER NOT NULL DEFAULT 0, ip TEXT NOT NULL DEFAULT '', like_count INTEGER NOT NULL DEFAULT 0, comment_count INTEGER NOT NULL DEFAULT 0, status INTEGER NOT NULL DEFAULT 1, created_time DATETIME NOT NULL, updated_time DATETIME NOT NULL)`,
		`CREATE TABLE comment_event_outbox (event_id TEXT PRIMARY KEY, aggregate_id INTEGER NOT NULL, event_type TEXT NOT NULL, version INTEGER NOT NULL, occurred_at DATETIME NOT NULL, payload TEXT NOT NULL, status INTEGER NOT NULL DEFAULT 0, attempts INTEGER NOT NULL DEFAULT 0, next_attempt_time DATETIME NOT NULL, published_time DATETIME NULL, last_error TEXT NOT NULL DEFAULT '', created_time DATETIME NOT NULL, updated_time DATETIME NOT NULL)`,
	} {
		if _, err := engine.Exec(statement); err != nil {
			engine.Close()
			t.Fatal(err)
		}
	}
	if _, err := engine.Exec(`INSERT INTO comments (id, article_id, user_id, content, root_id, like_count, status, created_time, updated_time) VALUES (1, 1, 7, '根评论', 0, 4, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		engine.Close()
		t.Fatal(err)
	}
	return &Repository{client: engine, transaction: engine, articleLocker: sqliteArticleLocker{engine: engine}}, engine
}

// sqliteArticleLocker 使用 SQLite 查询模拟事务内文章校验。
type sqliteArticleLocker struct {
	engine *xorm.Engine // engine 是隔离的评论测试数据库。
}

// LockPublishedArticle 模拟当前评论测试所需的依赖行为。
func (l sqliteArticleLocker) LockPublishedArticle(session *xorm.Session, articleID uint64) error {
	// 1. 执行当前测试依赖操作并记录断言数据
	var row struct {
		Status int8 `xorm:"status"` // Status 是测试文章状态：3-已发表。
	}
	found, err := session.Table("articles").Where("id = ?", articleID).Get(&row)
	if err != nil {
		return err
	}
	if !found || row.Status != comment.StatusPublished {
		return comment.ErrArticleNotPublished
	}
	return nil
}

// TestCreateWritesCommentCreatedOutboxInSameTransaction 验证评论创建同时写入集成事件。
func TestCreateWritesCommentCreatedOutboxInSameTransaction(t *testing.T) {
	// 1. 创建评论并查询同事务生成的 Outbox
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	created := &entity.Comment{ArticleID: 1, UserID: 9, Content: "新增评论", Status: comment.StatusNormal, CreatedTime: time.Now(), UpdatedTime: time.Now()}
	if err := repository.Create(context.Background(), created); err != nil {
		t.Fatal(err)
	}
	var count int64
	found, err := engine.SQL("SELECT COUNT(*) FROM comment_event_outbox WHERE aggregate_id = ? AND event_type = ?", created.ID, comment.CommentCreatedEventType).Get(&count)
	if err != nil || !found || count != 1 {
		t.Fatalf("outbox count = %d, found=%v, error=%v", count, found, err)
	}
}

// TestDeleteRootCascadesAndIsIdempotent 验证删除主评论级联回复且重复删除不重复产生事件。
func TestDeleteRootCascadesAndIsIdempotent(t *testing.T) {
	// 1. 准备主评论、两条直属回复和另一楼评论
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	for _, row := range []string{
		`INSERT INTO comments (id, article_id, user_id, content, root_id, status, created_time, updated_time) VALUES (2, 1, 8, '回复一', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO comments (id, article_id, user_id, content, root_id, status, created_time, updated_time) VALUES (3, 1, 9, '回复二', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO comments (id, article_id, user_id, content, root_id, status, created_time, updated_time) VALUES (4, 1, 10, '另一楼', 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`UPDATE comments SET comment_count = 2 WHERE id = 1`,
	} {
		if _, err := engine.Exec(row); err != nil {
			t.Fatal(err)
		}
	}

	// 2. 首次删除只软删除目标楼并为三条状态变化写事件
	if err := repository.Delete(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	var normalCount int64
	if _, err := engine.SQL("SELECT COUNT(*) FROM comments WHERE id IN (1,2,3) AND status = 1").Get(&normalCount); err != nil {
		t.Fatal(err)
	}
	if normalCount != 0 {
		t.Fatalf("normal count = %d, want 0", normalCount)
	}
	var otherStatus int
	if _, err := engine.SQL("SELECT status FROM comments WHERE id = 4").Get(&otherStatus); err != nil || otherStatus != int(comment.StatusNormal) {
		t.Fatalf("other status = %d, error=%v", otherStatus, err)
	}
	var eventCount int64
	if _, err := engine.SQL("SELECT COUNT(*) FROM comment_event_outbox WHERE event_type = ?", comment.CommentDeletedEventType).Get(&eventCount); err != nil || eventCount != 3 {
		t.Fatalf("delete events = %d, error=%v", eventCount, err)
	}

	// 3. 重复删除成功且不新增事件
	if err := repository.Delete(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	var repeatedCount int64
	if _, err := engine.SQL("SELECT COUNT(*) FROM comment_event_outbox WHERE event_type = ?", comment.CommentDeletedEventType).Get(&repeatedCount); err != nil || repeatedCount != eventCount {
		t.Fatalf("repeated events = %d, want %d, error=%v", repeatedCount, eventCount, err)
	}
}

// TestDeleteReplyOnlyAffectsItselfAndDecrementsRootOnce 验证回复删除不影响同楼其他回复。
func TestDeleteReplyOnlyAffectsItselfAndDecrementsRootOnce(t *testing.T) {
	// 1. 准备两条回复并将根回复数设为2
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	for _, row := range []string{
		`INSERT INTO comments (id, article_id, user_id, content, root_id, status, created_time, updated_time) VALUES (2, 1, 8, '回复一', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO comments (id, article_id, user_id, content, root_id, status, created_time, updated_time) VALUES (3, 1, 9, '回复二', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`UPDATE comments SET comment_count = 2 WHERE id = 1`,
	} {
		if _, err := engine.Exec(row); err != nil {
			t.Fatal(err)
		}
	}

	// 2. 删除一条回复后仅该回复隐藏且根计数只减一次
	if err := repository.Delete(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	var rootCount int64
	if _, err := engine.SQL("SELECT comment_count FROM comments WHERE id = 1").Get(&rootCount); err != nil || rootCount != 1 {
		t.Fatalf("root count = %d, error=%v", rootCount, err)
	}
	var siblingStatus int
	if _, err := engine.SQL("SELECT status FROM comments WHERE id = 3").Get(&siblingStatus); err != nil || siblingStatus != int(comment.StatusNormal) {
		t.Fatalf("sibling status = %d, error=%v", siblingStatus, err)
	}
	var eventCount int64
	if _, err := engine.SQL("SELECT COUNT(*) FROM comment_event_outbox WHERE event_type = ?", comment.CommentDeletedEventType).Get(&eventCount); err != nil || eventCount != 1 {
		t.Fatalf("delete events = %d, error=%v", eventCount, err)
	}
}

// TestCreateRollsBackWhenOutboxInsertFails 验证 Outbox 失败不会留下评论事实或回复计数。
func TestCreateRollsBackWhenOutboxInsertFails(t *testing.T) {
	// 1. 删除 Outbox 表模拟事务事件写入失败
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	if _, err := engine.Exec("DROP TABLE comment_event_outbox"); err != nil {
		t.Fatal(err)
	}
	reply := &entity.Comment{ArticleID: 1, UserID: 8, RootID: 1, Content: "应回滚回复", Status: comment.StatusNormal, CreatedTime: time.Now(), UpdatedTime: time.Now()}
	if err := repository.Create(context.Background(), reply); err == nil {
		t.Fatal("outbox insert failure did not roll back create")
	}

	// 2. 评论插入和根回复数增加均随事务回滚
	var replyCount int64
	if _, err := engine.SQL("SELECT COUNT(*) FROM comments WHERE root_id = 1").Get(&replyCount); err != nil || replyCount != 0 {
		t.Fatalf("reply rows = %d, error=%v", replyCount, err)
	}
	var rootCount int64
	if _, err := engine.SQL("SELECT comment_count FROM comments WHERE id = 1").Get(&rootCount); err != nil || rootCount != 0 {
		t.Fatalf("root count = %d, error=%v", rootCount, err)
	}
}

// TestDeleteRollsBackWhenOutboxInsertFails 验证删除事件写入失败时恢复评论状态和回复数。
func TestDeleteRollsBackWhenOutboxInsertFails(t *testing.T) {
	// 1. 准备回复并删除 Outbox 表制造事件写入失败
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	if _, err := engine.Exec(`INSERT INTO comments (id, article_id, user_id, content, root_id, status, created_time, updated_time) VALUES (2, 1, 8, '回复', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec("UPDATE comments SET comment_count = 1 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec("DROP TABLE comment_event_outbox"); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), 2); err == nil {
		t.Fatal("outbox insert failure did not roll back delete")
	}

	// 2. 回复状态和根回复数均保持删除前值
	var status int
	if _, err := engine.SQL("SELECT status FROM comments WHERE id = 2").Get(&status); err != nil || status != int(comment.StatusNormal) {
		t.Fatalf("reply status = %d, error=%v", status, err)
	}
	var rootCount int64
	if _, err := engine.SQL("SELECT comment_count FROM comments WHERE id = 1").Get(&rootCount); err != nil || rootCount != 1 {
		t.Fatalf("root count = %d, error=%v", rootCount, err)
	}
}
