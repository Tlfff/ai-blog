package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	notificationapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/notification"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"github.com/gin-gonic/gin"
)

// fakeNotificationUseCase 记录通知 HTTP 服务传入的认证用户。
type fakeNotificationUseCase struct {
	query      notification.PageQuery // query 是最近一次列表查询。
	unreadUser uint64                 // unreadUser 是未读统计接收者。
	readUser   uint64                 // readUser 是全部已读接收者。
}

// ConsumeArticleLike 满足通知完整用例接口。
func (f *fakeNotificationUseCase) ConsumeArticleLike(context.Context, like.IntegrationEvent) error {
	return nil
}

// List 记录列表查询并返回包含存量类型的通知。
func (f *fakeNotificationUseCase) List(_ context.Context, q notification.PageQuery) (*notification.ListResult, error) {
	f.query = q
	return &notification.ListResult{Page: 1, PageSize: 10, Items: []*entity.Notification{{ID: "id", Type: 2, SenderID: 8, CreatedTime: time.Unix(10, 0)}}}, nil
}

// CountUnread 记录接收者并返回预设数量。
func (f *fakeNotificationUseCase) CountUnread(_ context.Context, id uint64) (int64, error) {
	f.unreadUser = id
	return 4, nil
}

// MarkAllRead 记录被修改的接收者。
func (f *fakeNotificationUseCase) MarkAllRead(_ context.Context, id uint64) error {
	f.readUser = id
	return nil
}

// TestNotificationEndpointsUseAuthenticatedReceiver 验证三个接口只使用认证用户且兼容存量类型文案。
func TestNotificationEndpointsUseAuthenticatedReceiver(t *testing.T) {
	useCase := &fakeNotificationUseCase{}
	server := NewNotificationServer(useCase).(*NotificationService)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request, _ = http.NewRequest(http.MethodGet, "/auth/ntf/list", nil)
	identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7})
	list, err := server.ListNotifications(ctx, &notificationapi.ListNotificationsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if useCase.query.UserID != 7 || list.List[0].ActionText != "赞了你的评论" {
		t.Fatalf("query=%#v list=%#v", useCase.query, list)
	}
	if _, err := server.GetUnreadCount(ctx, &notificationapi.EmptyRequest{}); err != nil {
		t.Fatal(err)
	}
	override, ok := httpresponse.DataOverride(ctx)
	if !ok || string(override) != "4" || useCase.unreadUser != 7 {
		t.Fatalf("override=%s user=%d", override, useCase.unreadUser)
	}
	if _, err := server.MarkAllRead(ctx, &notificationapi.EmptyRequest{}); err != nil || useCase.readUser != 7 {
		t.Fatalf("read user=%d err=%v", useCase.readUser, err)
	}
}

// TestNotificationActionTextSupportsLegacyTypes 验证类型1～4均可转换为兼容展示文案。
func TestNotificationActionTextSupportsLegacyTypes(t *testing.T) {
	// 1. 新类型1和存量类型2～4都返回稳定非空文案
	for notificationType := int8(1); notificationType <= 4; notificationType++ {
		if text := notificationActionText(notificationType); text == "" || text == "与你产生了互动" {
			t.Fatalf("type=%d text=%q", notificationType, text)
		}
	}
}
