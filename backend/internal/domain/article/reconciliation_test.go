package article

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeInteractionProjectionRepository struct {
	snapshot InteractionProjectionSnapshot
	err      error
}

func (f *fakeInteractionProjectionRepository) RepairInteractionProjections(_ context.Context, snapshot InteractionProjectionSnapshot) error {
	f.snapshot = snapshot
	return f.err
}

func TestInteractionProjectionServiceValidatesAndDelegatesSnapshot(t *testing.T) {
	repository := &fakeInteractionProjectionRepository{}
	service := NewInteractionProjectionService(repository)
	snapshot := InteractionProjectionSnapshot{
		CapturedAt: time.Now(),
		Comments:   []CommentProjectionFact{{CommentID: 1, ArticleID: 2, Version: 1, Active: true}},
		Likes:      []LikeProjectionFact{{LikeID: 3, ArticleID: 2, UserID: 4, Version: 2}},
	}
	if err := service.RepairInteractionProjections(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if repository.snapshot.CapturedAt != snapshot.CapturedAt || len(repository.snapshot.Comments) != 1 || len(repository.snapshot.Likes) != 1 {
		t.Fatalf("delegated snapshot = %#v", repository.snapshot)
	}
}

func TestInteractionProjectionServiceRejectsDuplicateOrIncompleteFacts(t *testing.T) {
	service := NewInteractionProjectionService(&fakeInteractionProjectionRepository{})
	cases := []InteractionProjectionSnapshot{
		{},
		{CapturedAt: time.Now(), Comments: []CommentProjectionFact{{CommentID: 1, ArticleID: 2, Version: 1}, {CommentID: 1, ArticleID: 2, Version: 2}}},
		{CapturedAt: time.Now(), Likes: []LikeProjectionFact{{LikeID: 1, ArticleID: 2, Version: 1}}},
	}
	for _, snapshot := range cases {
		if err := service.RepairInteractionProjections(context.Background(), snapshot); !errors.Is(err, ErrInvalidInteractionProjectionSnapshot) {
			t.Fatalf("snapshot=%#v error=%v", snapshot, err)
		}
	}
}
