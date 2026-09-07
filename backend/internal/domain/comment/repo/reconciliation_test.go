package repo

import (
	"context"
	"testing"
)

func TestListArticleProjectionFactsUsesCommentStatusAndLatestVersion(t *testing.T) {
	repository, engine := newCommentTestRepository(t)
	defer engine.Close()
	for _, statement := range []string{
		`INSERT INTO comments (id, article_id, user_id, content, root_id, status, created_time, updated_time) VALUES (2, 1, 8, '已删除', 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO comment_event_outbox (event_id, aggregate_id, event_type, version, occurred_at, payload, status, attempts, next_attempt_time, last_error, created_time, updated_time) VALUES ('delete-2', 2, 'comment.deleted', 2, CURRENT_TIMESTAMP, '{}', 1, 0, CURRENT_TIMESTAMP, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	} {
		if _, err := engine.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	facts, err := repository.ListArticleProjectionFacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 || facts[0].CommentID != 1 || facts[0].Version != 1 || !facts[0].Active || facts[1].CommentID != 2 || facts[1].Version != 2 || facts[1].Active {
		t.Fatalf("facts=%#v", facts)
	}
}
