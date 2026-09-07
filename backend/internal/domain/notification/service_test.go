package notification

import (
	"context"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification/entity"
)

// fakeRepository 记录通知领域服务的仓储调用。
type fakeRepository struct {
	items      map[string]*entity.Notification // items 按事件标识保存通知。
	query      PageQuery                       // query 是最近一次列表分页参数。
	unreadUser uint64                          // unreadUser 是未读统计接收者。
	readUser   uint64                          // readUser 是全部已读接收者。
}

// Create 按事件标识幂等保存通知。
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

// List 记录分页参数并返回类型1～4通知。
func (f *fakeRepository) List(_ context.Context, q PageQuery) (*ListResult, error) {
	f.query = q
	return &ListResult{Items: []*entity.Notification{{Type: 1}, {Type: 2}, {Type: 3}, {Type: 4}}, Page: q.Page, PageSize: q.PageSize}, nil
}

// CountUnread 记录接收者并返回预设未读数量。
func (f *fakeRepository) CountUnread(_ context.Context, id uint64) (int64, error) {
	f.unreadUser = id
	return 3, nil
}

// MarkAllRead 记录被标记已读的接收者。
func (f *fakeRepository) MarkAllRead(_ context.Context, id uint64) error { f.readUser = id; return nil }

// fakeArticleReader 返回预设文章通知快照。
type fakeArticleReader struct {
	snapshot *ArticleSnapshot // snapshot 是预设文章通知快照。
}

// FindArticleSnapshot 返回预设文章通知快照。
func (f fakeArticleReader) FindArticleSnapshot(context.Context, uint64) (*ArticleSnapshot, error) {
	return f.snapshot, nil
}

// fakeUserReader 返回预设通知发送者快照。
type fakeUserReader struct {
	snapshot *SenderSnapshot // snapshot 是预设发送者快照。
}

// FindSenderSnapshot 返回预设通知发送者快照。
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
