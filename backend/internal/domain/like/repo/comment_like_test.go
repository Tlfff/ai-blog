package repo

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"context"
	"testing"
	"time"
)

// TestChangeCommentLikeIsIdempotentAndAtomic 验证评论重复操作幂等且事实与 Outbox 同事务。
func TestChangeCommentLikeIsIdempotentAndAtomic(t *testing.T) {
	repository, engine := newLikeTestRepository(t)
	defer engine.Close()
	now := time.Now()
	changed, err := repository.ChangeCommentLike(context.Background(), 7, 9, like.StatusLiked, now)
	if err != nil || !changed {
		t.Fatalf("first like=%v err=%v", changed, err)
	}
	changed, err = repository.ChangeCommentLike(context.Background(), 7, 9, like.StatusLiked, now)
	if err != nil || changed {
		t.Fatalf("duplicate like=%v err=%v", changed, err)
	}
	assertTableCount(t, engine, "comment_likes", 1)
	assertTableCount(t, engine, "comment_like_event_outbox", 1)
	changed, err = repository.ChangeCommentLike(context.Background(), 7, 9, like.StatusUnliked, now)
	if err != nil || !changed {
		t.Fatalf("cancel=%v err=%v", changed, err)
	}
	changed, err = repository.ChangeCommentLike(context.Background(), 7, 9, like.StatusUnliked, now)
	if err != nil || changed {
		t.Fatalf("duplicate cancel=%v err=%v", changed, err)
	}
	assertTableCount(t, engine, "comment_like_event_outbox", 2)
}

// TestChangeCommentLikeRollsBackWithoutOutbox 验证评论点赞事实与 Outbox 原子提交。
func TestChangeCommentLikeRollsBackWithoutOutbox(t *testing.T) {
	repository, engine := newLikeTestRepository(t)
	defer engine.Close()
	_, _ = engine.Exec("DROP TABLE comment_like_event_outbox")
	if _, err := repository.ChangeCommentLike(context.Background(), 7, 9, like.StatusLiked, time.Now()); err == nil {
		t.Fatal("expected error")
	}
	assertTableCount(t, engine, "comment_likes", 0)
}
