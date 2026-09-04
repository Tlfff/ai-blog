package repo

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

// TestNewRepositoryRequiresTransactions 验证文章仓储禁止非事务客户端降级。
func TestNewRepositoryRequiresTransactions(t *testing.T) {
	// 1. 仅具备普通 XORM 能力的客户端必须在启动阶段失败
	var client clients.MysqlClient = new(nonTransactionalClient)
	defer func() {
		if recover() == nil {
			t.Fatal("NewRepository() did not panic for non-transactional client")
		}
	}()
	NewRepository(client, nil)
}

// nonTransactionalClient 用于表达不具备 Transaction 的测试客户端。
type nonTransactionalClient struct {
	clients.MysqlClient // MysqlClient 提供普通数据库接口，但运行时值不实现事务扩展。
}

// TestUpdateArticleSynchronizesImagesAtomically 验证文章字段和图片绑定关系在同一事务更新。
func TestUpdateArticleSynchronizesImagesAtomically(t *testing.T) {
	// 1. 创建包含当前图片、未绑定图片和其他文章图片的数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)

	// 2. 更新文章时保留外部图片文本、绑定新增系统图片并解绑移除图片
	updatedTime := time.Now().Truncate(time.Second)
	updated := &entity.Article{
		ID: 1, AuthorID: 7, Title: "新标题", Content: "![新图](image://11) ![外部图](https://example.test/a.png)",
		Tags: []string{"Go"}, Status: article.StatusPublished, UpdatedTime: updatedTime,
	}
	err := repository.UpdateArticle(context.Background(), 1, []uint64{11}, replaceArticleWith(updated))
	if err != nil {
		t.Fatal(err)
	}

	// 3. 校验文章更新和图片关系同步结果
	var persisted struct {
		Title       string    `xorm:"'title'"`        // Title 是更新后的文章标题。
		Content     string    `xorm:"'content'"`      // Content 是保留外部图片地址的正文。
		Tags        string    `xorm:"'tags'"`         // Tags 是更新后的标签文本。
		Status      int       `xorm:"'status'"`       // Status 是更新后的文章状态。
		UpdatedTime time.Time `xorm:"'updated_time'"` // UpdatedTime 是更新后的修改时间。
	}
	found, err := engine.SQL("SELECT title, content, tags, status, updated_time FROM articles WHERE id = 1").Get(&persisted)
	if err != nil || !found {
		t.Fatalf("query article: found=%v error=%v", found, err)
	}
	if persisted.Title != "新标题" || persisted.Content != updated.Content || persisted.Tags != "Go" ||
		persisted.Status != int(article.StatusPublished) || !persisted.UpdatedTime.Equal(updatedTime) {
		t.Fatalf("article = %#v", persisted)
	}
	assertImageOwner(t, engine, 10, nil)
	articleID := uint64(1)
	assertImageOwner(t, engine, 11, &articleID)
}

// TestUpdateArticleRollsBackWhenImageBelongsToAnotherArticle 验证图片冲突时文章和关系均不改变。
func TestUpdateArticleRollsBackWhenImageBelongsToAnotherArticle(t *testing.T) {
	// 1. 创建包含其他文章已绑定图片的数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)

	// 2. 尝试将其他文章图片绑定到当前文章
	updated := &entity.Article{
		ID: 1, AuthorID: 7, Title: "不应保存", Content: "![冲突图](image://12)", Status: article.StatusDraft, UpdatedTime: time.Now(),
	}
	err := repository.UpdateArticle(context.Background(), 1, []uint64{12}, replaceArticleWith(updated))
	if !errors.Is(err, article.ErrImageAlreadyBound) {
		t.Fatalf("UpdateArticle() error = %v", err)
	}

	// 3. 校验事务回滚后标题和原图片归属保持不变
	var title string
	if _, err := engine.SQL("SELECT title FROM articles WHERE id = 1").Get(&title); err != nil {
		t.Fatal(err)
	}
	if title != "原标题" {
		t.Fatalf("title = %q", title)
	}
	articleID := uint64(1)
	assertImageOwner(t, engine, 10, &articleID)
}

// TestUpdateArticleRollsBackWhenImageIsMissing 验证图片不存在时文章和关系均不改变。
func TestUpdateArticleRollsBackWhenImageIsMissing(t *testing.T) {
	// 1. 创建只有当前文章原图片的数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)

	// 2. 尝试绑定不存在的正文图片
	updated := &entity.Article{
		ID: 1, AuthorID: 7, Title: "不应保存", Content: "![缺失图](image://99)", Status: article.StatusDraft, UpdatedTime: time.Now(),
	}
	err := repository.UpdateArticle(context.Background(), 1, []uint64{99}, replaceArticleWith(updated))
	if !errors.Is(err, article.ErrImageNotFound) {
		t.Fatalf("UpdateArticle() error = %v", err)
	}

	// 3. 校验事务回滚后标题和原图片归属保持不变
	var title string
	if _, err := engine.SQL("SELECT title FROM articles WHERE id = 1").Get(&title); err != nil {
		t.Fatal(err)
	}
	if title != "原标题" {
		t.Fatalf("title = %q", title)
	}
	articleID := uint64(1)
	assertImageOwner(t, engine, 10, &articleID)
}

// TestArticleMutationRollsBackWhenDomainRejects 验证领域规则拒绝时事务不写入文章。
func TestArticleMutationRollsBackWhenDomainRejects(t *testing.T) {
	// 1. 创建待更新的草稿文章数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)

	// 2. 领域 mutation 返回错误时更新和发布均原样返回
	rejected := func(*entity.Article) error {
		return article.ErrArticleNotOwned
	}
	if err := repository.UpdateArticle(context.Background(), 1, nil, rejected); !errors.Is(err, article.ErrArticleNotOwned) {
		t.Fatalf("UpdateArticle() error = %v", err)
	}
	if err := repository.ChangeArticleStatus(context.Background(), 1, rejected); !errors.Is(err, article.ErrArticleNotOwned) {
		t.Fatalf("ChangeArticleStatus() error = %v", err)
	}

	// 3. 校验拒绝后文章标题和状态均未改变
	var persisted struct {
		Title  string `xorm:"'title'"`  // Title 是事务拒绝后的文章标题。
		Status int    `xorm:"'status'"` // Status 是事务拒绝后的文章状态。
	}
	found, err := engine.SQL("SELECT title, status FROM articles WHERE id = 1").Get(&persisted)
	if err != nil || !found {
		t.Fatalf("query article: found=%v error=%v", found, err)
	}
	if persisted.Title != "原标题" || persisted.Status != int(article.StatusDraft) {
		t.Fatalf("article = %#v", persisted)
	}
}

// TestFindPublicDetailOnlyReturnsPublishedArticle 验证公开详情只读取已发表文章。
func TestFindPublicDetailOnlyReturnsPublishedArticle(t *testing.T) {
	// 1. 创建包含草稿和已发表文章的数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)

	// 2. 已发表文章返回作者和图片映射
	detail, err := repository.FindPublicDetail(context.Background(), 4)
	if err != nil || detail.Article == nil || detail.Article.Status != article.StatusPublished || detail.AuthorNickname != "作者" {
		t.Fatalf("detail = %#v, error = %v", detail, err)
	}

	// 3. 草稿和不存在文章返回稳定领域错误
	if _, err := repository.FindPublicDetail(context.Background(), 1); !errors.Is(err, article.ErrArticleNotPublished) {
		t.Fatalf("draft detail error = %v", err)
	}
	if _, err := repository.FindPublicDetail(context.Background(), 99); !errors.Is(err, article.ErrArticleNotFound) {
		t.Fatalf("missing detail error = %v", err)
	}
}

// TestPublishArticleUpdatesStatusAndTimestamp 验证发布写入搜索同步依赖的状态和修改时间。
func TestPublishArticleUpdatesStatusAndTimestamp(t *testing.T) {
	// 1. 创建草稿文章数据库夹具并确定发布修改时间
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)
	updatedTime := time.Now().Add(time.Hour).Truncate(time.Second)

	// 2. 作者发布自己的草稿文章
	if err := repository.ChangeArticleStatus(context.Background(), 1, publishArticleAt(updatedTime)); err != nil {
		t.Fatal(err)
	}

	// 3. 状态和 updated_time 必须同时更新以产生完整 articles 表 Binlog
	var persisted struct {
		Status      int       `xorm:"'status'"`       // Status 是发布后的文章状态。
		UpdatedTime time.Time `xorm:"'updated_time'"` // UpdatedTime 是发布后的修改时间。
	}
	found, err := engine.SQL("SELECT status, updated_time FROM articles WHERE id = 1").Get(&persisted)
	if err != nil || !found {
		t.Fatalf("query article: found=%v error=%v", found, err)
	}
	if persisted.Status != int(article.StatusPublished) || !persisted.UpdatedTime.Equal(updatedTime) {
		t.Fatalf("status = %d, updated time = %v", persisted.Status, persisted.UpdatedTime)
	}
}

// TestListArticlesSupportsStatusCursorAndOffsetPagination 验证作者筛选及两种分页模式。
func TestListArticlesSupportsStatusCursorAndOffsetPagination(t *testing.T) {
	// 1. 创建包含多个作者和状态的文章数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)

	// 2. 倒序 Offset 首页只返回当前作者并提供完整筛选总数
	firstPage, err := repository.ListArticles(context.Background(), article.ListQuery{
		AuthorID: 7, Status: article.StatusAll, Page: 1, PageSize: 2, IsDesc: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Articles) != 2 || firstPage.Articles[0].ID != 4 || firstPage.Articles[1].ID != 3 || firstPage.LastID != 3 || firstPage.Total != 3 {
		t.Fatalf("first page = %#v", firstPage)
	}

	// 3. last_id 大于零时优先使用游标并沿相同方向继续查询
	cursorPage, err := repository.ListArticles(context.Background(), article.ListQuery{
		AuthorID: 7, Status: article.StatusAll, LastID: firstPage.LastID, Page: 99, PageSize: 2, IsDesc: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cursorPage.Articles) != 1 || cursorPage.Articles[0].ID != 1 || cursorPage.Total != 3 {
		t.Fatalf("cursor page = %#v", cursorPage)
	}

	// 4. last_id 为空时按 page 计算 Offset
	offsetPage, err := repository.ListArticles(context.Background(), article.ListQuery{
		AuthorID: 7, Status: article.StatusAll, Page: 2, PageSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offsetPage.Articles) != 1 || offsetPage.Articles[0].ID != 4 || offsetPage.Total != 3 {
		t.Fatalf("offset page = %#v", offsetPage)
	}

	// 5. 非删除筛选遵循兼容状态语义
	notDeleted, err := repository.ListArticles(context.Background(), article.ListQuery{
		AuthorID: 7, Status: article.StatusNotDeleted, Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notDeleted.Articles) != 2 || notDeleted.Articles[0].ID != 1 || notDeleted.Articles[1].ID != 4 || notDeleted.Total != 2 {
		t.Fatalf("not deleted = %#v", notDeleted)
	}
}

// TestClearArticleKeepsDatabaseRowsWhenRemovalFails 验证对象删除失败时事务保留数据库记录。
func TestClearArticleKeepsDatabaseRowsWhenRemovalFails(t *testing.T) {
	// 1. 创建包含绑定图片的垃圾箱文章数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)

	// 2. 模拟 MinIO 对象删除失败
	err := repository.ClearArticle(context.Background(), 3, func(_ *entity.Article, images []*entity.Image) error {
		if len(images) != 1 || images[0].ObjectKey != "trash.png" {
			t.Fatalf("images = %#v", images)
		}
		return errors.New("minio unavailable")
	})
	if err == nil {
		t.Fatal("ClearArticle() error = nil")
	}

	// 3. 事务回滚后文章和图片记录均存在
	assertRowCount(t, engine, "articles", "id = ?", 3, 1)
	assertRowCount(t, engine, "article_images", "id = ?", 13, 1)
}

// TestClearArticleDeletesImageAndArticleRowsAfterRemoval 验证对象删除成功后事务硬删除数据库记录。
func TestClearArticleDeletesImageAndArticleRowsAfterRemoval(t *testing.T) {
	// 1. 创建包含绑定图片的垃圾箱文章数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)

	// 2. 对象删除回调成功后执行数据库硬删除
	if err := repository.ClearArticle(context.Background(), 3, func(current *entity.Article, images []*entity.Image) error {
		if current.Status != article.StatusDeleted || len(images) != 1 {
			t.Fatalf("article = %#v, images = %#v", current, images)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// 3. 图片记录和文章记录均已删除
	assertRowCount(t, engine, "articles", "id = ?", 3, 0)
	assertRowCount(t, engine, "article_images", "id = ?", 13, 0)
}

// TestPublishedReadingRepositoryMaintainsHistoryAndViewCount 验证公开列表及浏览统计事务。
func TestPublishedReadingRepositoryMaintainsHistoryAndViewCount(t *testing.T) {
	// 1. 创建包含草稿和已发表文章的数据库夹具
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)
	now := time.Now().Truncate(time.Second)
	if _, err := engine.Exec("INSERT INTO articles (id, author_id, title, content, tags, status, created_time, updated_time) VALUES (?, ?, ?, ?, '', ?, ?, ?)",
		5, 8, "另一篇已发表", "正文", article.StatusPublished, now, now); err != nil {
		t.Fatal(err)
	}

	// 2. 公开列表仅返回已发表文章，并支持倒序 Offset 首页
	articles, total, err := repository.ListPublished(context.Background(), article.PublicListQuery{Page: 1, PageSize: 1, IsDesc: true})
	if err != nil || total != 2 || len(articles) != 1 || articles[0].ID != 5 {
		t.Fatalf("articles = %#v, total = %d, error = %v", articles, total, err)
	}
	cursorArticles, _, err := repository.ListPublished(context.Background(), article.PublicListQuery{LastID: 5, Page: 99, PageSize: 1, IsDesc: true})
	if err != nil || len(cursorArticles) != 1 || cursorArticles[0].ID != 4 {
		t.Fatalf("cursor articles = %#v, error = %v", cursorArticles, err)
	}

	// 3. 游客和登录用户均增加浏览量，登录用户重复浏览只保留一条历史
	if _, err := repository.RecordView(context.Background(), article.ViewEvent{EventID: "guest", ArticleID: 4, ViewedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.RecordView(context.Background(), article.ViewEvent{EventID: "user-1", ArticleID: 4, UserID: 7, ViewedAt: now}); err != nil {
		t.Fatal(err)
	}
	if metric, err := repository.RecordView(context.Background(), article.ViewEvent{EventID: "user-2", ArticleID: 4, UserID: 7, ViewedAt: now.Add(time.Minute)}); err != nil || metric.ViewCount != 3 {
		t.Fatalf("metric = %#v, error = %v", metric, err)
	}
	if metric, err := repository.RecordView(context.Background(), article.ViewEvent{EventID: "user-2", ArticleID: 4, UserID: 7, ViewedAt: now.Add(time.Minute)}); err != nil || metric.ViewCount != 3 {
		t.Fatalf("duplicate metric = %#v, error = %v", metric, err)
	}
	assertRowCount(t, engine, "article_view_histories", "user_id = ?", 7, 1)
}

// replaceArticleWith 创建覆盖文章可编辑字段的测试领域 mutation。
func replaceArticleWith(updated *entity.Article) article.ArticleMutation {
	// 1. 复制更新接口允许修改的字段
	return func(current *entity.Article) error {
		current.Title = updated.Title
		current.Content = updated.Content
		current.Tags = append([]string(nil), updated.Tags...)
		current.Status = updated.Status
		current.UpdatedTime = updated.UpdatedTime
		return nil
	}
}

// publishArticleAt 创建写入发表状态和修改时间的测试领域 mutation。
func publishArticleAt(updatedTime time.Time) article.ArticleMutation {
	// 1. 设置发布所需的状态和修改时间
	return func(current *entity.Article) error {
		current.Status = article.StatusPublished
		current.UpdatedTime = updatedTime
		return nil
	}
}

// newArticleTestRepository 创建文章仓储使用的隔离内存数据库。
func newArticleTestRepository(t *testing.T) (*Repository, *xorm.Engine) {
	// 1. 使用测试名称隔离共享内存数据库
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	engine, err := xorm.NewEngine("sqlite3", dsn)
	if err != nil {
		t.Fatalf("create database engine: %v", err)
	}
	schema := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, nickname TEXT NOT NULL, avatar TEXT NULL, last_login_ip TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE articles (id INTEGER PRIMARY KEY, author_id INTEGER NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL, tags TEXT NOT NULL, status INTEGER NOT NULL, view_count INTEGER NOT NULL DEFAULT 0, like_count INTEGER NOT NULL DEFAULT 0, comment_count INTEGER NOT NULL DEFAULT 0, created_time DATETIME NOT NULL, updated_time DATETIME NOT NULL)`,
		`CREATE TABLE article_images (id INTEGER PRIMARY KEY, article_id INTEGER NULL, object_key TEXT NOT NULL, created_time DATETIME NOT NULL)`,
		`CREATE TABLE article_view_histories (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, article_id INTEGER NOT NULL, created_time DATETIME NOT NULL, updated_time DATETIME NOT NULL)`,
		`CREATE TABLE article_view_event_inbox (event_id TEXT PRIMARY KEY, article_id INTEGER NOT NULL, processed_time DATETIME NOT NULL)`,
	}
	for _, statement := range schema {
		if _, err := engine.Exec(statement); err != nil {
			closeArticleTestEngine(t, engine)
			t.Fatalf("create schema: %v", err)
		}
	}
	now := time.Now().Truncate(time.Second)
	if _, err := engine.Exec("INSERT INTO users (id, nickname, avatar, last_login_ip) VALUES (7, '作者', '', '203.0.113.8'), (8, '其他作者', '', '')"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec("INSERT INTO articles (id, author_id, title, content, tags, status, created_time, updated_time) VALUES (?, ?, ?, ?, '', ?, ?, ?), (?, ?, ?, ?, '', ?, ?, ?), (?, ?, ?, ?, '', ?, ?, ?), (?, ?, ?, ?, '', ?, ?, ?)",
		1, 7, "原标题", "旧正文", article.StatusDraft, now, now,
		2, 8, "其他文章", "正文", article.StatusDraft, now, now,
		3, 7, "已删除", "正文", article.StatusDeleted, now, now,
		4, 7, "已发表", "正文", article.StatusPublished, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec("INSERT INTO article_images (id, article_id, object_key, created_time) VALUES (?, ?, ?, ?), (?, NULL, ?, ?), (?, ?, ?, ?), (?, ?, ?, ?)",
		10, 1, "old.png", now, 11, "new.png", now, 12, 2, "other.png", now, 13, 3, "trash.png", now); err != nil {
		t.Fatal(err)
	}
	return &Repository{client: engine, transaction: engine}, engine
}

// assertRowCount 校验指定条件对应的数据库记录数量。
//
// 参数说明：
//   - t：当前测试上下文。
//   - engine：隔离的测试数据库引擎。
//   - table：待统计的数据库表名。
//   - condition：带单个文章或图片标识占位符的查询条件。
//   - id：查询条件使用的文章或图片标识。
//   - want：预期记录数量。
func assertRowCount(t *testing.T, engine *xorm.Engine, table, condition string, id uint64, want int64) {
	// 1. 查询指定表和条件的记录数量
	t.Helper()
	count, err := engine.Table(table).Where(condition, id).Count()
	if err != nil {
		t.Fatalf("count %s rows: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s row count = %d, want %d", table, count, want)
	}
}

// assertImageOwner 校验正文图片当前文章归属。
func assertImageOwner(t *testing.T, engine *xorm.Engine, imageID uint64, wantArticleID *uint64) {
	// 1. 查询可空文章标识并与预期关系比较
	t.Helper()
	var actual struct {
		ArticleID *uint64 `xorm:"'article_id'"` // ArticleID 是图片当前所属文章标识。
	}
	found, err := engine.SQL("SELECT article_id FROM article_images WHERE id = ?", imageID).Get(&actual)
	if err != nil || !found {
		t.Fatalf("query image owner: found=%v error=%v", found, err)
	}
	if wantArticleID == nil {
		if actual.ArticleID != nil {
			t.Fatalf("image %d article ID = %v, want nil", imageID, *actual.ArticleID)
		}
		return
	}
	if actual.ArticleID == nil || *actual.ArticleID != *wantArticleID {
		t.Fatalf("image %d article ID = %v, want %d", imageID, actual.ArticleID, *wantArticleID)
	}
}

// closeArticleTestEngine 关闭文章仓储测试数据库。
func closeArticleTestEngine(t *testing.T, engine *xorm.Engine) {
	// 1. 关闭数据库并报告资源释放错误
	t.Helper()
	if err := engine.Close(); err != nil {
		t.Errorf("close database engine: %v", err)
	}
}
