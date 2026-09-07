package repo

import (
	"context"
	"errors"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
)

// fakeArticleNotificationRepository 提供文章通知快照测试结果。
type fakeArticleNotificationRepository struct {
	snapshot *article.NotificationSnapshot // snapshot 是预设文章快照。
	err      error                         // err 是预设查询错误。
}

// FindNotificationSnapshot 返回预设文章快照或错误。
func (f fakeArticleNotificationRepository) FindNotificationSnapshot(context.Context, uint64) (*article.NotificationSnapshot, error) {
	return f.snapshot, f.err
}

// TestArticleReaderTreatsMissingArticleAsPermanentInvalidSnapshot 验证缺失文章不会被当作瞬时依赖故障无限重投。
func TestArticleReaderTreatsMissingArticleAsPermanentInvalidSnapshot(t *testing.T) {
	// 1. 文章不存在时适配器返回空快照，由通知领域归类为无效事件
	reader := NewArticleReader(article.NewNotificationQuery(fakeArticleNotificationRepository{err: article.ErrArticleNotFound}))
	snapshot, err := reader.FindArticleSnapshot(context.Background(), 9)
	if err != nil || snapshot != nil {
		t.Fatalf("snapshot=%#v err=%v", snapshot, err)
	}

	// 2. 其他基础设施错误仍向上返回，以便 Kafka 后续重投
	wantErr := errors.New("mysql unavailable")
	reader = NewArticleReader(article.NewNotificationQuery(fakeArticleNotificationRepository{err: wantErr}))
	if _, err := reader.FindArticleSnapshot(context.Background(), 9); !errors.Is(err, wantErr) {
		t.Fatalf("err=%v", err)
	}
}
