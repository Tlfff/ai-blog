package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification/entity"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var errMissingMongoClient = errors.New("通知 MongoDB 仓储缺少客户端")

const (
	notificationCollection = "notifications"
	maxMongoSkip           = uint64(1<<63 - 1)
)

// document 表示 MongoDB 中兼容类型1～4的通知文档。
type document struct {
	ID             bson.ObjectID `bson:"_id,omitempty"`             // ID 是 MongoDB 文档标识。
	SourceEventID  string        `bson:"source_event_id,omitempty"` // SourceEventID 是新建通知的幂等事件标识。
	ReceiverID     uint64        `bson:"receiver_id"`               // ReceiverID 是通知接收者标识。
	Type           int8          `bson:"type"`                      // Type 是通知类型：1～4。
	IsRead         bool          `bson:"is_read"`                   // IsRead 表示接收者是否已读。
	CreatedTime    time.Time     `bson:"created_time"`              // CreatedTime 是通知创建时间。
	SenderID       uint64        `bson:"sender_id"`                 // SenderID 是发送者标识。
	SenderNickname string        `bson:"sender_nickname"`           // SenderNickname 是发送者昵称快照。
	SenderAvatar   string        `bson:"sender_avatar"`             // SenderAvatar 是发送者头像快照。
	ArticleID      uint64        `bson:"article_id,omitempty"`      // ArticleID 是关联文章标识。
	Title          string        `bson:"title,omitempty"`           // Title 是文章标题快照。
}

// Repository 使用 MongoDB 保存通知文档。
type Repository struct {
	collection *mongo.Collection // collection 是通知文档集合。
}

// NewRepository 创建通知 MongoDB 仓储并确保必要索引。
func NewRepository(client *clients.MongoClient) (*Repository, error) {
	// 1. 启动阶段拒绝缺少 MongoDB 客户端
	if client == nil || client.Client == nil || client.Database == "" {
		return nil, errMissingMongoClient
	}
	collection := client.Client.Database(client.Database).Collection(notificationCollection)

	// 2. 创建事件幂等、接收者分页和未读计数索引
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := collection.Indexes().CreateMany(ctx, notificationIndexes())
	if err != nil {
		return nil, fmt.Errorf("创建通知 MongoDB 索引: %w", err)
	}
	return &Repository{collection: collection}, nil
}

// Create 幂等保存通知快照。
func (r *Repository) Create(ctx context.Context, item *entity.Notification) error {
	// 1. 将领域快照完整写入通知文档
	if item == nil {
		return notification.ErrInvalidInput
	}
	_, err := r.collection.InsertOne(ctx, document{
		SourceEventID: item.SourceEventID, ReceiverID: item.ReceiverID, Type: item.Type,
		IsRead: item.IsRead, CreatedTime: item.CreatedTime, SenderID: item.SenderID,
		SenderNickname: item.SenderNickname, SenderAvatar: item.SenderAvatar,
		ArticleID: item.ArticleID, Title: item.Title,
	})
	// 2. 唯一事件索引冲突表示消息重复，按幂等成功处理
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("写入通知文档: %w", err)
	}
	return nil
}

// List 按接收者和时间倒序分页查询通知。
func (r *Repository) List(ctx context.Context, query notification.PageQuery) (*notification.ListResult, error) {
	// 1. 拒绝 MongoDB int64 skip 无法表达的极端页码
	skip, err := notificationSkip(query)
	if err != nil {
		return nil, err
	}

	// 2. 过滤当前接收者并按时间、文档标识稳定倒序分页
	limit := int64(query.PageSize) // #nosec G115 -- notificationSkip 已验证 PageSize 不超过 int64 上限。
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_time", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)
	cursor, err := r.collection.Find(ctx, receiverFilter(query.UserID), findOptions)
	if err != nil {
		return nil, fmt.Errorf("查询通知文档: %w", err)
	}
	defer cursor.Close(ctx)
	rows := make([]document, 0, query.PageSize)
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("读取通知文档: %w", err)
	}

	// 3. 不过滤类型字段，兼容读取类型2～4的存量文档
	items := make([]*entity.Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, notificationFromDocument(row))
	}
	return &notification.ListResult{Items: items, Page: query.Page, PageSize: query.PageSize}, nil
}

// notificationSkip 将 Offset 分页参数安全转换为 MongoDB int64 skip。
func notificationSkip(query notification.PageQuery) (int64, error) {
	// 1. 仓储拒绝未规范化参数及乘法溢出
	if query.Page == 0 || query.PageSize == 0 || query.PageSize > maxMongoSkip || query.Page-1 > maxMongoSkip/query.PageSize {
		return 0, notification.ErrInvalidInput
	}
	skip := int64((query.Page - 1) * query.PageSize) // #nosec G115 -- 上述边界检查保证结果不超过 int64。
	return skip, nil
}

// CountUnread 统计当前接收者未读通知。
func (r *Repository) CountUnread(ctx context.Context, userID uint64) (int64, error) {
	// 1. 接收者条件防止跨用户读取未读数量
	count, err := r.collection.CountDocuments(ctx, unreadFilter(userID))
	if err != nil {
		return 0, fmt.Errorf("统计未读通知文档: %w", err)
	}
	return count, nil
}

// MarkAllRead 仅更新当前接收者未读通知。
func (r *Repository) MarkAllRead(ctx context.Context, userID uint64) error {
	// 1. 接收者和未读条件共同限制批量更新范围
	_, err := r.collection.UpdateMany(ctx, unreadFilter(userID), bson.M{"$set": bson.M{"is_read": true}})
	if err != nil {
		return fmt.Errorf("更新未读通知文档: %w", err)
	}
	return nil
}

// notificationIndexes 返回通知幂等和接收者查询索引。
func notificationIndexes() []mongo.IndexModel {
	// 1. 稀疏唯一索引兼容没有 source_event_id 的类型2～4存量文档
	return []mongo.IndexModel{
		{Keys: bson.D{{Key: "source_event_id", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "receiver_id", Value: 1}, {Key: "created_time", Value: -1}, {Key: "_id", Value: -1}}},
		{Keys: bson.D{{Key: "receiver_id", Value: 1}, {Key: "is_read", Value: 1}}},
	}
}

// receiverFilter 返回严格限定接收者的查询条件。
func receiverFilter(userID uint64) bson.M {
	// 1. 列表查询始终限定认证接收者
	return bson.M{"receiver_id": userID}
}

// unreadFilter 返回严格限定接收者和未读状态的查询条件。
func unreadFilter(userID uint64) bson.M {
	// 1. 未读统计和更新同时限定认证接收者与未读状态
	return bson.M{"receiver_id": userID, "is_read": false}
}

// notificationFromDocument 将类型1～4 MongoDB 文档转换为领域通知。
func notificationFromDocument(row document) *entity.Notification {
	// 1. 保留存量类型及其已有快照字段，不补造跨上下文数据
	return &entity.Notification{
		ID: row.ID.Hex(), SourceEventID: row.SourceEventID, ReceiverID: row.ReceiverID,
		Type: row.Type, IsRead: row.IsRead, CreatedTime: row.CreatedTime,
		SenderID: row.SenderID, SenderNickname: row.SenderNickname, SenderAvatar: row.SenderAvatar,
		ArticleID: row.ArticleID, Title: row.Title,
	}
}
