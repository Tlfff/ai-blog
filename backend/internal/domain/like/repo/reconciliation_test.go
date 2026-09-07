package repo

import (
	"context"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
)

func TestListArticleProjectionFactsUsesLikeFactsAndLatestVersion(t *testing.T) {
	repository, engine := newLikeTestRepository(t)
	defer engine.Close()
	if _, err := repository.ChangeArticleLike(context.Background(), 7, 9, like.StatusLiked, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ChangeArticleLike(context.Background(), 8, 9, like.StatusLiked, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ChangeArticleLike(context.Background(), 8, 9, like.StatusUnliked, time.Now()); err != nil {
		t.Fatal(err)
	}
	facts, err := repository.ListArticleProjectionFacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 || facts[0].UserID != 7 || facts[0].Version != 1 || !facts[0].Active || facts[1].UserID != 8 || facts[1].Version != 2 || facts[1].Active {
		t.Fatalf("facts=%#v", facts)
	}
}
