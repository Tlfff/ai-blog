package repo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"xorm.io/xorm"
)

func TestRepairInteractionProjectionsRebuildsCountsAndRelationshipState(t *testing.T) {
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)
	old := time.Now().Add(-time.Hour)
	for _, statement := range []string{
		`UPDATE articles SET comment_count = 99, like_count = 88 WHERE id = 4`,
		`INSERT INTO article_comment_projection (comment_id, article_id, version, active, last_event_id, updated_time) VALUES (999, 4, 9, 1, 'stale', ?)`,
		`INSERT INTO article_like_projection (like_id, article_id, user_id, version, active, last_event_id, updated_time) VALUES (999, 4, 9, 9, 1, 'stale', ?)`,
	} {
		if _, err := engine.Exec(statement, old); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := article.InteractionProjectionSnapshot{
		CapturedAt: time.Now(),
		Comments: []article.CommentProjectionFact{
			{CommentID: 101, ArticleID: 4, Version: 1, Active: true},
			{CommentID: 102, ArticleID: 4, Version: 2, Active: false},
		},
		Likes: []article.LikeProjectionFact{
			{LikeID: 201, ArticleID: 4, UserID: 7, Version: 1, Active: true},
			{LikeID: 202, ArticleID: 4, UserID: 8, Version: 2, Active: false},
		},
	}
	if err := repository.RepairInteractionProjections(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	var counts struct {
		CommentCount int64 `xorm:"'comment_count'"`
		LikeCount    int64 `xorm:"'like_count'"`
	}
	found, err := engine.SQL("SELECT comment_count, like_count FROM articles WHERE id = 4").Get(&counts)
	if err != nil || !found || counts.CommentCount != 1 || counts.LikeCount != 1 {
		t.Fatalf("counts=%#v found=%v error=%v", counts, found, err)
	}
	for table, id := range map[string]uint64{"article_comment_projection": 999, "article_like_projection": 999} {
		count, err := engine.Table(table).Where(map[string]any{mapProjectionIDColumn(table): id}).Count()
		if err != nil || count != 0 {
			t.Fatalf("stale %s count=%d error=%v", table, count, err)
		}
	}
	assertProjectionState(t, engine, "article_comment_projection", "comment_id", 101, 1, 1)
	assertProjectionState(t, engine, "article_comment_projection", "comment_id", 102, 2, 0)
	assertProjectionState(t, engine, "article_like_projection", "like_id", 201, 1, 1)
	assertProjectionState(t, engine, "article_like_projection", "like_id", 202, 2, 0)
}

func TestRepairInteractionProjectionsPreservesEventsNewerThanSnapshot(t *testing.T) {
	repository, engine := newArticleTestRepository(t)
	defer closeArticleTestEngine(t, engine)
	capturedAt := time.Now().Add(-time.Minute)
	if _, err := engine.Exec(`INSERT INTO article_like_projection (like_id, article_id, user_id, version, active, last_event_id, updated_time) VALUES (201, 4, 7, 3, 1, 'new-event', ?)`, time.Now()); err != nil {
		t.Fatal(err)
	}
	snapshot := article.InteractionProjectionSnapshot{
		CapturedAt: capturedAt,
		Likes:      []article.LikeProjectionFact{{LikeID: 201, ArticleID: 4, UserID: 7, Version: 2, Active: false}},
	}
	if err := repository.RepairInteractionProjections(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	assertProjectionState(t, engine, "article_like_projection", "like_id", 201, 3, 1)
	var count int64
	if _, err := engine.SQL("SELECT like_count FROM articles WHERE id = 4").Get(&count); err != nil || count != 1 {
		t.Fatalf("like_count=%d error=%v", count, err)
	}
}

func mapProjectionIDColumn(table string) string {
	if table == "article_comment_projection" {
		return "comment_id"
	}
	return "like_id"
}

func assertProjectionState(t *testing.T, engine *xorm.Engine, table, idColumn string, id uint64, wantVersion int64, wantActive int8) {
	t.Helper()
	var state struct {
		Version int64 `xorm:"'version'"`
		Active  int8  `xorm:"'active'"`
	}
	query := fmt.Sprintf("SELECT version, active FROM %s WHERE %s = ?", table, idColumn)
	found, err := engine.SQL(query, id).Get(&state)
	if err != nil || !found || state.Version != wantVersion || state.Active != wantActive {
		t.Fatalf("%s id=%d state=%#v found=%v error=%v", table, id, state, found, err)
	}
}
