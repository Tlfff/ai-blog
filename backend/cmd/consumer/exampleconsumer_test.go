package consumer

import (
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// TestMaxConsumerMessageBufferSizeIncludesNotification 验证通知消费配置参与共享 Streamer 缓冲计算。
func TestMaxConsumerMessageBufferSizeIncludesNotification(t *testing.T) {
	// 1. 通知缓冲最大时必须成为最终共享配置
	got := maxConsumerMessageBufferSize(
		&conf.KafkaConsumer_Config{MessageBufferSize: 1},
		&conf.KafkaConsumer_Config{MessageBufferSize: 2},
		&conf.KafkaConsumer_Config{MessageBufferSize: 3},
		&conf.KafkaConsumer_Config{MessageBufferSize: 16},
	)
	if got != 16 {
		t.Fatalf("message buffer size=%d", got)
	}
}
