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
		Status int8 `xorm:"status"`
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
