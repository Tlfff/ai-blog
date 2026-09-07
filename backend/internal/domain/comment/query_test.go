package comment

import (
	"context"
	"errors"
	"testing"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
)

// TestQueryServiceReturnsLikeCountAndHotValue 验证评论热度等于点赞数与回复数之和。
func TestQueryServiceReturnsLikeCountAndHotValue(t *testing.T) {
	// 1. 正常评论统计只由评论上下文实体字段组成
	service := NewQueryService(&fakeRepository{root: &entity.Comment{ID: 9, Status: StatusNormal, LikeCount: 4, ReplyCount: 3}})
	stats, err := service.GetStats(context.Background(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CommentID != 9 || stats.LikeCount != 4 || stats.ReplyCount != 3 || stats.HotValue() != 7 {
		t.Fatalf("stats = %#v, hot = %d", stats, stats.HotValue())
	}
}

// TestQueryServiceRejectsInvalidOrDeletedComment 验证无效标识和已删除评论不公开统计。
func TestQueryServiceRejectsInvalidOrDeletedComment(t *testing.T) {
	// 1. 无效标识返回参数错误
	service := NewQueryService(&fakeRepository{})
	if _, err := service.GetStats(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid id error = %v", err)
	}

	// 2. 已删除评论按不存在处理
	service = NewQueryService(&fakeRepository{root: &entity.Comment{ID: 9, Status: StatusDeleted}})
	if _, err := service.GetStats(context.Background(), 9); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("deleted comment error = %v", err)
	}
}
