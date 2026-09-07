package notification

import (
	"context"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification/entity"
)

type fakeRepository struct {
	items      map[string]*entity.Notification
	query      PageQuery
	unreadUser uint64
	readUser   uint64
}

func (f *fakeRepository) Create(_ context.Context, item *entity.Notification) error {
	if f.items == nil {
		f.items = map[string]*entity.Notification{}
	}
	if _, ok := f.items[item.SourceEventID]; !ok {
		copyItem := *item
		f.items[item.SourceEventID] = &copyItem
	}
	return nil
}
func (f *fakeRepository) List(_ context.Context, q PageQuery) (*ListResult, error) {
	f.query = q
	return &ListResult{Items: []*entity.Notification{{Type: 1}, {Type: 2}, {Type: 3}, {Type: 4}}, Page: q.Page, PageSize: q.PageSize}, nil
}
func (f *fakeRepository) CountUnread(_ context.Context, id uint64) (int64, error) {
	f.unreadUser = id
	return 3, nil
}
func (f *fakeRepository) MarkAllRead(_ context.Context, id uint64) error { f.readUser = id; return nil }

type fakeArticleReader struct{ snapshot *ArticleSnapshot }

func (f fakeArticleReader) FindArticleSnapshot(context.Context, uint64) (*ArticleSnapshot, error) {
	return f.snapshot, nil
}

type fakeUserReader struct{ snapshot *SenderSnapshot }

func (f fakeUserReader) FindSenderSnapshot(context.Context, uint64) (*SenderSnapshot, error) {
	return f.snapshot, nil
}

// TestServiceCreatesOneSnapshotNotificationForFirstExternalLike 验证首次他人点赞生成一条快照通知且重复事件幂等。
func TestServiceCreatesOneSnapshotNotificationForFirstExternalLike(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, fakeArticleReader{&ArticleSnapshot{ID: 9, AuthorID: 8, Title: "标题快照"}}, fakeUserReader{&SenderSnapshot{ID: 7, Nickname: "点赞者", Avatar: "/a.png"}})
	event := like.IntegrationEvent{EventID: "event-1", EventType: like.ArticleLikedEventType, Version: 1, ArticleID: 9, UserID: 7, OccurredAt: time.Now()}
	if err := service.ConsumeArticleLike(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := service.ConsumeArticleLike(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("items=%d", len(repo.items))
	}
	item := repo.items[event.EventID]
	if item.Type != TypeArticleLike || item.ReceiverID != 8 || item.SenderNickname != "点赞者" || item.SenderAvatar != "/a.png" || item.Title != "标题快照" {
		t.Fatalf("item=%#v", item)
	}
}

// TestServiceSkipsSelfLikeAndNonFirstEvents 验证作者自赞、取消和再次点赞不生成通知。
func TestServiceSkipsSelfLikeAndNonFirstEvents(t *testing.T) {
	repo := &fakeRepository{}
	self := NewService(repo, fakeArticleReader{&ArticleSnapshot{ID: 9, AuthorID: 7, Title: "标题"}}, fakeUserReader{&SenderSnapshot{ID: 7}})
	for _, event := range []like.IntegrationEvent{{EventID: "self", EventType: like.ArticleLikedEventType, Version: 1, ArticleID: 9, UserID: 7, OccurredAt: time.Now()}, {EventID: "cancel", EventType: like.ArticleUnlikedEventType, Version: 2, ArticleID: 9, UserID: 8}, {EventID: "relike", EventType: like.ArticleLikedEventType, Version: 3, ArticleID: 9, UserID: 8}} {
		if err := self.ConsumeArticleLike(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(repo.items) != 0 {
		t.Fatalf("items=%d", len(repo.items))
	}
}

// TestServiceScopesQueriesAndNormalizesPagination 验证用户隔离和功能文档分页回退规则。
func TestServiceScopesQueriesAndNormalizesPagination(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, fakeArticleReader{}, fakeUserReader{})
	result, err := service.List(context.Background(), PageQuery{UserID: 7, Page: 0, PageSize: 200})
	if err != nil {
		t.Fatal(err)
	}
	if repo.query.UserID != 7 || result.Page != 1 || result.PageSize != 10 || len(result.Items) != 4 {
		t.Fatalf("query=%#v result=%#v", repo.query, result)
	}
	count, _ := service.CountUnread(context.Background(), 7)
	if count != 3 || repo.unreadUser != 7 {
		t.Fatal("unread scope")
	}
	if err := service.MarkAllRead(context.Background(), 7); err != nil || repo.readUser != 7 {
		t.Fatal("read scope")
	}
}
