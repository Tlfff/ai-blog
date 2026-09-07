package job

import (
	"context"
	"errors"
	"testing"
)

// fakeTask 记录后台任务执行并返回预设错误。
type fakeTask struct {
	calls int   // calls 是任务执行次数。
	err   error // err 是任务预设错误。
}

// Job 执行测试任务。
func (f *fakeTask) Job() error {
	// 1. 记录单次执行并返回预设结果
	f.calls++
	return f.err
}

// TestBlogJobRunsOnceAndStopsWithContext 验证任务接入 Leo 取消信号。
func TestBlogJobRunsOnceAndStopsWithContext(t *testing.T) {
	// 1. 取消上下文后任务 Runner 应执行一次并正常退出
	current := &fakeTask{}
	job := newBlogJob(current)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := job.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if current.calls != 1 {
		t.Fatalf("calls = %d, want 1", current.calls)
	}
}

// TestBlogJobReturnsTaskFailure 验证任务错误交给 Leo 统一处理。
func TestBlogJobReturnsTaskFailure(t *testing.T) {
	// 1. 任务失败时保留原始错误链且不等待取消
	want := errors.New("job failed")
	current := &fakeTask{err: want}
	if err := newBlogJob(current).Run(context.Background()); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}
