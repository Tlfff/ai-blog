//go:build wireinject
// +build wireinject

package consumer

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/consumer"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain"
	"codeup.aliyun.com/qimao/leo/leo/stream"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	conf.ProviderSet,
	consumer.ProviderConsumerSet,
	// 基础层
	clients.ProviderClientsSet,
	// 领域层
	domain.DomainProviderAppSet,
	newBlogStreamer,
)

func newBlogStreamerApp() (*stream.Streamer, func(), error) {
	panic(wire.Build(
		ProviderSet,
	))
}
