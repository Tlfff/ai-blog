package clients

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	errMissingMongoConfig  = errors.New("通知 MongoDB 配置缺失")
	errEmptyMongoTarget    = errors.New("通知 MongoDB URI 或数据库名为空")
	errInvalidMongoTimeout = errors.New("通知 MongoDB 连接超时配置无效")
)

// MongoClient 聚合通知 MongoDB 客户端与数据库名。
type MongoClient struct {
	Client   *mongo.Client // Client 是由进程生命周期统一关闭的 MongoDB 客户端。
	Database string        // Database 是通知集合所属数据库名。
}

// NewMongoClient 创建并验证通知 MongoDB 连接。
func NewMongoClient(config *conf.Config) (*MongoClient, func(), error) {
	// 1. 校验通知文档存储配置
	if config == nil || config.GetData() == nil || config.GetData().GetMongo() == nil {
		return nil, nil, errMissingMongoConfig
	}
	mongoConfig := config.GetData().GetMongo()
	if mongoConfig.GetUri() == "" || mongoConfig.GetDatabase() == "" {
		return nil, nil, errEmptyMongoTarget
	}
	timeout := 5 * time.Second
	if mongoConfig.GetConnectTimeout() != "" {
		parsed, err := time.ParseDuration(mongoConfig.GetConnectTimeout())
		if err != nil || parsed <= 0 {
			return nil, nil, errInvalidMongoTimeout
		}
		timeout = parsed
	}

	// 2. 建立连接并在启动阶段执行 Ping
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(mongoConfig.GetUri()))
	if err != nil {
		return nil, nil, fmt.Errorf("连接通知 MongoDB: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("探测通知 MongoDB: %w", err)
	}

	// 3. 返回由 Wire 清理链统一关闭的客户端
	cleanup := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = client.Disconnect(shutdownCtx)
	}
	return &MongoClient{Client: client, Database: mongoConfig.GetDatabase()}, cleanup, nil
}
