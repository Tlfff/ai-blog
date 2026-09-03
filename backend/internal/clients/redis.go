package clients

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/leo/leo/config"
	"codeup.aliyun.com/qimao/leo/leo/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

// NewRedis
func NewRedisClient() (RedisClient, func(), error) {
	rconf := conf.Redis{}
	if err := config.Get("data.redis.blog").Scan(&rconf); err != nil {
		return nil, nil, err
	}
	client := redis.NewClient(newRedisConfig(&rconf))
	ctx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelFunc()
	err := client.Ping(ctx).Err()
	logger := log.L()
	if err != nil {
		logger.Error(err.Error())
		return nil, nil, err
	}
	logger.Info("  redis connect success")
	return client, func() {
		client.Close()
	}, nil
}

func newRedisConfig(c *conf.Redis) *redis.Options {
	readTimeout, _ := time.ParseDuration(c.ReadTimeout)
	writeTimeout, _ := time.ParseDuration(c.WriteTimeout)
	dialTimeout, _ := time.ParseDuration(c.DialTimeout)
	return &redis.Options{
		Addr:         c.Addr,
		Password:     c.Password,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		DialTimeout:  dialTimeout,
		DB:           int(c.Db),
	}
}
