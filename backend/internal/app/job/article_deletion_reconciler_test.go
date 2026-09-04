package job

import (
	"context"
	"testing"
	"time"
)

// fakeDeletionRecovery 记录周期恢复任务调用。
type fakeDeletionRecovery struct {
	called chan struct{} // called 在恢复任务执行时发送通知。
}

// ReconcileStagedDeletions 记录一次正文图片暂存删除恢复。
func (f *fakeDeletionRecovery) ReconcileStagedDeletions(context.Context) error {
	// 1. 非阻塞通知测试恢复任务已执行
	select {
	case f.called <- struct{}{}:
	default:
	}
	return nil
}

// TestArticleDeletionReconcilerRunsPeriodically 验证恢复任务周期执行并响应退出。
func TestArticleDeletionReconcilerRunsPeriodically(t *testing.T) {
	// 1. 使用短间隔启动由 Leo 生命周期兼容的恢复 Runner
	recovery := &fakeDeletionRecovery{called: make(chan struct{}, 1)}
	reconciler := newArticleDeletionReconciler(recovery, time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- reconciler.Run(ctx)
	}()

	// 2. 等待至少一次周期恢复后取消 Runner
	select {
	case <-recovery.called:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("ArticleDeletionReconciler did not run")
	}

	// 3. Runner 必须在上下文取消后正常退出
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ArticleDeletionReconciler did not stop")
	}
}
