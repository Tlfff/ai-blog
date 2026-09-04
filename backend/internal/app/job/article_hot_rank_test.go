package job

import (
	"context"
	"testing"
	"time"
)

// fakeHotRankRebuilder 记录热榜重建调用。
type fakeHotRankRebuilder struct {
	called chan struct{} // called 在热榜重建时发送通知。
}

// RebuildHotRank 记录一次权威热榜重建。
func (f *fakeHotRankRebuilder) RebuildHotRank(context.Context) error {
	// 1. 非阻塞通知测试重建已执行
	select {
	case f.called <- struct{}{}:
	default:
	}
	return nil
}

// TestArticleHotRankJobRebuildsAtStartup 验证 HTTP 服务启动时立即初始化热榜。
func TestArticleHotRankJobRebuildsAtStartup(t *testing.T) {
	// 1. 启动热榜 Runner 并等待首次重建
	rebuilder := &fakeHotRankRebuilder{called: make(chan struct{}, 1)}
	job := NewArticleHotRankJob(rebuilder)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- job.Run(ctx) }()
	select {
	case <-rebuilder.called:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("ArticleHotRankJob did not rebuild at startup")
	}

	// 2. 上下文取消后 Runner 正常退出
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ArticleHotRankJob did not stop")
	}
}
