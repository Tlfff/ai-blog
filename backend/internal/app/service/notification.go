package service

import (
	"errors"
	"strconv"

	notificationapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/notification"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/httpresponse"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"codeup.aliyun.com/qimao/leo/lib/errassets"
	"github.com/gin-gonic/gin"
)

const notificationUnauthenticatedCode = 44030101

// NotificationService 将通知 HTTP 协议适配到通知上下文公开接口。
type NotificationService struct {
	useCase notification.UseCase // useCase 提供通知查询和已读操作。
}

// NewNotificationServer 创建通知 HTTP 服务。
func NewNotificationServer(useCase notification.UseCase) notificationapi.NotificationServiceHTTPServerController {
	// 1. 启动阶段拒绝缺少通知领域服务
	if useCase == nil {
		panic("通知 HTTP 服务缺少领域服务")
	}
	return &NotificationService{useCase: useCase}
}

// ListNotifications 查询当前用户通知列表。
func (s *NotificationService) ListNotifications(ctx *gin.Context, request *notificationapi.ListNotificationsRequest) (*notificationapi.NotificationListReply, error) {
	// 1. 只使用认证上下文中的当前用户作为接收者
	current, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(notificationUnauthenticatedCode, "未登录")
	}
	result, err := s.useCase.List(ctx.Request.Context(), notification.PageQuery{UserID: current.ID, Page: request.GetPage(), PageSize: request.GetPageSize()})
	if err != nil {
		return nil, notificationHTTPError(err)
	}

	// 2. 转换类型1～4的兼容展示文案和快照字段
	items := make([]*notificationapi.NotificationItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, &notificationapi.NotificationItem{
			Id: item.ID, Type: int32(item.Type), IsRead: item.IsRead,
			CreatedTime: item.CreatedTime.Unix(), SenderId: item.SenderID,
			SenderNickname: item.SenderNickname, SenderAvatar: item.SenderAvatar,
			ActionText: notificationActionText(item.Type), ArticleId: item.ArticleID, Title: item.Title,
		})
	}
	return &notificationapi.NotificationListReply{List: items, Page: result.Page, PageSize: result.PageSize}, nil
}

// GetUnreadCount 查询当前用户未读通知数。
func (s *NotificationService) GetUnreadCount(ctx *gin.Context, _ *notificationapi.EmptyRequest) (*notificationapi.UnreadCountReply, error) {
	// 1. 只按认证用户统计未读通知
	current, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(notificationUnauthenticatedCode, "未登录")
	}
	count, err := s.useCase.CountUnread(ctx.Request.Context(), current.ID)
	if err != nil {
		return nil, notificationHTTPError(err)
	}

	// 2. 覆盖生成响应为功能文档约定的 data=int64
	httpresponse.SetDataOverride(ctx, []byte(strconv.FormatInt(count, 10)))
	return &notificationapi.UnreadCountReply{Count: count}, nil
}

// MarkAllRead 将当前用户全部通知标记为已读。
func (s *NotificationService) MarkAllRead(ctx *gin.Context, _ *notificationapi.EmptyRequest) (*notificationapi.EmptyReply, error) {
	// 1. 只修改认证用户自己的通知
	current, ok := identity.FromContext(ctx)
	if !ok {
		return nil, errassets.NewError(notificationUnauthenticatedCode, "未登录")
	}
	if err := s.useCase.MarkAllRead(ctx.Request.Context(), current.ID); err != nil {
		return nil, notificationHTTPError(err)
	}
	httpresponse.SetSuccess(ctx, "全部标记为已读成功", true)
	return &notificationapi.EmptyReply{}, nil
}

// notificationActionText 返回类型1～4的兼容展示文案。
func notificationActionText(notificationType int8) string {
	// 1. 保留存量类型的读取语义，只主动创建类型1
	switch notificationType {
	case 1:
		return "赞了你的文章"
	case 2:
		return "赞了你的评论"
	case 3:
		return "评论了你的文章"
	case 4:
		return "回复了你的评论"
	default:
		return "与你产生了互动"
	}
}

// notificationHTTPError 将通知领域错误转换为稳定业务码。
func notificationHTTPError(err error) error {
	// 1. 仅转换可预期参数错误，依赖错误保留原始错误链
	if errors.Is(err, notification.ErrInvalidInput) {
		return errassets.NewError(44010104, "通知参数不合法")
	}
	return err
}
