package repo

import (
	"testing"

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
