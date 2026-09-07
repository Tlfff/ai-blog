package repo

import (
	"errors"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TestNotificationIndexesAndFiltersPreserveIdempotencyAndUserIsolation 验证唯一事件索引及接收者过滤条件。
func TestNotificationIndexesAndFiltersPreserveIdempotencyAndUserIsolation(t *testing.T) {
	indexes := notificationIndexes()
	if len(indexes) != 3 {
		t.Fatalf("indexes=%d", len(indexes))
	}
	if indexes[0].Options == nil {
		t.Fatal("missing unique sparse index options")
	}
	configured := new(options.IndexOptions)
	for _, apply := range indexes[0].Options.List() {
		if err := apply(configured); err != nil {
			t.Fatal(err)
		}
	}
	if configured.Unique == nil || !*configured.Unique || configured.Sparse == nil || !*configured.Sparse {
		t.Fatalf("index options=%#v", configured)
	}
	filter := unreadFilter(7)
	if filter["receiver_id"] != uint64(7) || filter["is_read"] != false {
		t.Fatalf("filter=%#v", filter)
	}
	if _, ok := receiverFilter(8)["type"]; ok {
		t.Fatal("list must not filter legacy types")
	}
	_ = bson.M{}
}

// TestNotificationFromDocumentPreservesLegacyTypes 验证类型2～4存量文档可直接读取。
func TestNotificationFromDocumentPreservesLegacyTypes(t *testing.T) {
	// 1. 转换不得过滤或改写存量通知类型
	for notificationType := int8(2); notificationType <= 4; notificationType++ {
		item := notificationFromDocument(document{Type: notificationType, ReceiverID: 7, SenderNickname: "存量用户"})
		if item.Type != notificationType || item.ReceiverID != 7 || item.SenderNickname != "存量用户" {
			t.Fatalf("item=%#v", item)
		}
	}
}

// TestNotificationSkipRejectsOverflow 验证极端页码不会回绕为负数 MongoDB skip。
func TestNotificationSkipRejectsOverflow(t *testing.T) {
	// 1. 正常分页计算稳定 Offset
	skip, err := notificationSkip(notification.PageQuery{Page: 3, PageSize: 10})
	if err != nil || skip != 20 {
		t.Fatalf("skip=%d err=%v", skip, err)
	}

	// 2. 超过 int64 上限的分页参数按业务参数错误返回
	for _, query := range []notification.PageQuery{
		{Page: ^uint64(0), PageSize: 10},
		{Page: 1, PageSize: ^uint64(0)},
	} {
		if _, err := notificationSkip(query); !errors.Is(err, notification.ErrInvalidInput) {
			t.Fatalf("query=%#v err=%v", query, err)
		}
	}
}
