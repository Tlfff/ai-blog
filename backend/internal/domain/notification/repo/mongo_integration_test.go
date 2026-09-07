package repo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification/entity"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TestMongoRepositoryIntegration 验证真实 MongoDB 上的幂等写入、用户隔离、存量读取和已读更新。
func TestMongoRepositoryIntegration(t *testing.T) {
	// 1. 仅在显式提供隔离测试 MongoDB 时运行真实持久化验收
	uri := os.Getenv("NOTIFICATION_MONGO_URI")
	if uri == "" {
		t.Skip("NOTIFICATION_MONGO_URI is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("disconnect mongo: %v", err)
		}
	}()
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}
	database := fmt.Sprintf("notification_test_%d", time.Now().UnixNano())
	defer func() {
		if err := client.Database(database).Drop(context.Background()); err != nil {
			t.Errorf("drop database: %v", err)
		}
	}()
	repository, err := NewRepository(&clients.MongoClient{Client: client, Database: database})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 同一事件重复写入只保留一条完整快照通知
	createdAt := time.Unix(100, 0).UTC()
	item := &entity.Notification{
		SourceEventID: "event-1", ReceiverID: 7, Type: notification.TypeArticleLike,
		CreatedTime: createdAt, SenderID: 8, SenderNickname: "发送者", SenderAvatar: "avatar",
		ArticleID: 9, Title: "文章标题",
	}
	if err := repository.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	// 3. 插入类型2～4存量文档和其他接收者，验证读取与权限过滤
	legacy := []any{
		document{ReceiverID: 7, Type: 2, CreatedTime: createdAt.Add(time.Second), SenderID: 10, SenderNickname: "类型2"},
		document{ReceiverID: 7, Type: 3, CreatedTime: createdAt.Add(2 * time.Second), SenderID: 11, SenderNickname: "类型3"},
		document{ReceiverID: 7, Type: 4, CreatedTime: createdAt.Add(3 * time.Second), SenderID: 12, SenderNickname: "类型4"},
		document{ReceiverID: 8, Type: 1, CreatedTime: createdAt, SenderID: 7, SenderNickname: "其他接收者"},
	}
	if _, err := repository.collection.InsertMany(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	result, err := repository.List(ctx, notification.PageQuery{UserID: 7, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 4 || result.Items[3].SenderNickname != "发送者" || result.Items[3].Title != "文章标题" {
		t.Fatalf("result=%#v", result)
	}
	for index, notificationType := range []int8{4, 3, 2, 1} {
		if result.Items[index].ReceiverID != 7 || result.Items[index].Type != notificationType {
			t.Fatalf("item[%d]=%#v", index, result.Items[index])
		}
	}

	// 4. 未读统计和批量已读只影响当前接收者
	count, err := repository.CountUnread(ctx, 7)
	if err != nil || count != 4 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := repository.MarkAllRead(ctx, 7); err != nil {
		t.Fatal(err)
	}
	count, err = repository.CountUnread(ctx, 7)
	if err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	otherCount, err := repository.CountUnread(ctx, 8)
	if err != nil || otherCount != 1 {
		t.Fatalf("other count=%d err=%v", otherCount, err)
	}
}
