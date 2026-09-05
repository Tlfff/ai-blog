package article

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
)

// fakeReadingRepository 记录公开阅读领域服务的权威数据调用。
type fakeReadingRepository struct {
	articles    []*entity.Article // articles 是公开列表预设文章。
	recordCalls int               // recordCalls 是浏览写入调用次数。
	metric      *HotMetric        // metric 是热榜权威统计。
	processed   bool              // processed 表示 MySQL Inbox 已提交事件。
}

// ListPublished 返回测试公开文章列表。
func (f *fakeReadingRepository) ListPublished(context.Context, PublicListQuery) ([]*entity.Article, uint64, error) {
	// 1. 返回预设文章及其总数
	return f.articles, uint64(len(f.articles)), nil
}

// RecordView 记录一次浏览写入。
func (f *fakeReadingRepository) RecordView(context.Context, ViewEvent) (*HotMetric, error) {
	// 1. 累加写入次数并返回权威统计
	f.recordCalls++
	return f.metric, nil
}

// ViewEventProcessed 返回测试 Inbox 状态。
func (f *fakeReadingRepository) ViewEventProcessed(context.Context, string) (bool, error) {
	// 1. 返回预设 MySQL 事务完成状态
	return f.processed, nil
}

// FindHotMetric 返回单篇文章权威统计。
func (f *fakeReadingRepository) FindHotMetric(context.Context, uint64) (*HotMetric, error) {
	// 1. 返回预设权威统计
	return f.metric, nil
}

// FindHotMetrics 返回批量权威统计。
func (f *fakeReadingRepository) FindHotMetrics(context.Context, []uint64) ([]*HotMetric, error) {
	// 1. 返回预设权威统计集合
	return []*HotMetric{f.metric}, nil
}

// TopHotMetrics 返回热度最高的权威统计。
func (f *fakeReadingRepository) TopHotMetrics(context.Context, int) ([]*HotMetric, error) {
	// 1. 返回预设权威统计集合
	return []*HotMetric{f.metric}, nil
}

// fakeViewPublisher 模拟浏览事件发布器。
type fakeViewPublisher struct{}

// PublishView 模拟成功发布浏览事件。
func (fakeViewPublisher) PublishView(context.Context, ViewEvent) error {
	// 1. 领域测试不访问真实 Kafka
	return nil
}

// fakeDedupe 返回浏览事件幂等状态。
type fakeDedupe struct {
	states         []ViewEventState // states 是依次返回的事件状态。
	completeErrors []error          // completeErrors 是依次返回的完成标记错误。
	completed      int              // completed 是完成标记写入次数。
}

// Begin 返回下一项事件处理状态。
func (f *fakeDedupe) Begin(context.Context, string, time.Duration) (ViewEventState, error) {
	// 1. 取出预设状态并推进队列
	state := f.states[0]
	f.states = f.states[1:]
	return state, nil
}

// Complete 记录事件完成标记。
func (f *fakeDedupe) Complete(context.Context, string, time.Duration) error {
	// 1. 累加完成标记次数
	f.completed++
	if len(f.completeErrors) > 0 {
		err := f.completeErrors[0]
		f.completeErrors = f.completeErrors[1:]
		return err
	}
	return nil
}

// Release 模拟释放事件处理权。
func (*fakeDedupe) Release(context.Context, string) error {
	// 1. 测试默认成功释放
	return nil
}

// fakeHotRank 记录 Redis 热榜投影更新。
type fakeHotRank struct {
	scores  int         // scores 是热度覆盖次数。
	entries []RankEntry // entries 是热榜排名预设值。
}

// Top 返回测试热榜排名。
func (f *fakeHotRank) Top(context.Context, int64) ([]RankEntry, error) {
	// 1. 返回预设排名
	return f.entries, nil
}

// SetScore 记录权威热度覆盖。
func (f *fakeHotRank) SetScore(context.Context, uint64, int64) error {
	// 1. 累加投影覆盖次数
	f.scores++
	return nil
}

// Replace 模拟热榜原子重建。
func (*fakeHotRank) Replace(context.Context, []*HotMetric) error {
	// 1. 测试默认成功替换
	return nil
}

// TestListPublishedTruncatesUnicodeSummary 验证公开列表按 Unicode 字符生成摘要。
func TestListPublishedTruncatesUnicodeSummary(t *testing.T) {
	// 1. 构造超过五十个 Unicode 字符的正文
	content := "这是中文摘要" + string(make([]rune, 46))
	repository := &fakeReadingRepository{articles: []*entity.Article{{ID: 1, Content: content}}}
	service := NewViewService(repository, fakeViewPublisher{}, &fakeDedupe{}, &fakeHotRank{})
	result, err := service.ListPublished(context.Background(), PublicListCommand{})
	if err != nil || result.Page != 1 || result.PageSize != 10 || len([]rune(result.Items[0].Summary)) != 53 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

// TestConsumeViewIsIdempotentAndRepairsHotRank 验证重复事件不增加浏览量并修复热榜。
func TestConsumeViewIsIdempotentAndRepairsHotRank(t *testing.T) {
	// 1. 同一事件首次处理后以 completed 状态重复投递
	metric := &HotMetric{ArticleID: 1, ViewCount: 3}
	repository := &fakeReadingRepository{metric: metric}
	dedupe := &fakeDedupe{states: []ViewEventState{ViewEventNew, ViewEventCompleted}}
	hotRank := &fakeHotRank{}
	service := NewViewService(repository, fakeViewPublisher{}, dedupe, hotRank)
	event := ViewEvent{EventID: "event", ArticleID: 1, ViewedAt: time.Now()}
	if err := service.ConsumeView(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := service.ConsumeView(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if repository.recordCalls != 1 || hotRank.scores != 2 || dedupe.completed != 2 {
		t.Fatalf("record calls=%d scores=%d completed=%d", repository.recordCalls, hotRank.scores, dedupe.completed)
	}
}

// TestConsumeViewRejectsConcurrentProcessing 验证处理中事件由 Kafka 重试。
func TestConsumeViewRejectsConcurrentProcessing(t *testing.T) {
	// 1. 处理中状态应返回可重试错误且不写入权威数据
	service := NewViewService(&fakeReadingRepository{}, fakeViewPublisher{}, &fakeDedupe{states: []ViewEventState{ViewEventProcessing}}, &fakeHotRank{})
	err := service.ConsumeView(context.Background(), ViewEvent{EventID: "event", ArticleID: 1, ViewedAt: time.Now()})
	if !errors.Is(err, ErrViewEventProcessing) {
		t.Fatalf("error = %v", err)
	}
}

// TestConsumeViewRepairsAfterRedisCompleteFailure 验证 MySQL Inbox 防止完成标记失败后重复计数。
func TestConsumeViewRepairsAfterRedisCompleteFailure(t *testing.T) {
	// 1. 首次数据库事务成功，但 Redis 完成标记失败
	metric := &HotMetric{ArticleID: 1, ViewCount: 3}
	repository := &fakeReadingRepository{metric: metric}
	dedupe := &fakeDedupe{states: []ViewEventState{ViewEventNew, ViewEventProcessing}, completeErrors: []error{errors.New("redis unavailable"), nil}}
	hotRank := &fakeHotRank{}
	service := NewViewService(repository, fakeViewPublisher{}, dedupe, hotRank)
	event := ViewEvent{EventID: "event", ArticleID: 1, ViewedAt: time.Now()}
	if err := service.ConsumeView(context.Background(), event); err == nil {
		t.Fatal("first ConsumeView() error = nil")
	}

	// 2. 重试发现 MySQL Inbox 已提交，只修复缓存而不再次增加浏览量
	repository.processed = true
	if err := service.ConsumeView(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if repository.recordCalls != 1 || hotRank.scores != 1 || dedupe.completed != 2 {
		t.Fatalf("record calls=%d scores=%d completed=%d", repository.recordCalls, hotRank.scores, dedupe.completed)
	}
}
