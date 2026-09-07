package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
)

type fakeCommentProjectionSource struct {
	facts []comment.ArticleProjectionFact
	err   error
}

func (f fakeCommentProjectionSource) ListArticleProjectionFacts(context.Context) ([]comment.ArticleProjectionFact, error) {
	return f.facts, f.err
}

type fakeLikeProjectionSource struct {
	facts []like.ArticleProjectionFact
	err   error
}

func (f fakeLikeProjectionSource) ListArticleProjectionFacts(context.Context) ([]like.ArticleProjectionFact, error) {
	return f.facts, f.err
}

type fakeInteractionRepairer struct {
	snapshot article.InteractionProjectionSnapshot
	called   int
}

func (f *fakeInteractionRepairer) RepairInteractionProjections(_ context.Context, snapshot article.InteractionProjectionSnapshot) error {
	f.called++
	f.snapshot = snapshot
	return nil
}

func TestArticleInteractionReconcileJobTranslatesStableContextSnapshots(t *testing.T) {
	repairer := &fakeInteractionRepairer{}
	capturedAt := time.Date(2026, time.September, 7, 15, 0, 0, 0, time.Local)
	job := NewArticleInteractionReconcileJob(
		fakeCommentProjectionSource{facts: []comment.ArticleProjectionFact{{CommentID: 1, ArticleID: 2, Version: 1, Active: true}}},
		fakeLikeProjectionSource{facts: []like.ArticleProjectionFact{{LikeID: 3, ArticleID: 2, UserID: 4, Version: 2}}},
		repairer,
	)
	job.now = func() time.Time { return capturedAt }
	if err := job.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repairer.called != 1 || repairer.snapshot.CapturedAt != capturedAt || len(repairer.snapshot.Comments) != 1 || len(repairer.snapshot.Likes) != 1 {
		t.Fatalf("called=%d snapshot=%#v", repairer.called, repairer.snapshot)
	}
}

func TestArticleInteractionReconcileJobStopsBeforeWriteWhenSourceFails(t *testing.T) {
	repairer := &fakeInteractionRepairer{}
	job := NewArticleInteractionReconcileJob(fakeCommentProjectionSource{err: errors.New("mysql unavailable")}, fakeLikeProjectionSource{}, repairer)
	if err := job.Reconcile(context.Background()); err == nil || repairer.called != 0 {
		t.Fatalf("error=%v called=%d", err, repairer.called)
	}
}
