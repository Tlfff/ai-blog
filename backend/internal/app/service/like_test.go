package service

import (
	"context"
	"net/http"
	"testing"

	likeapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/pkg/identity"
	"github.com/gin-gonic/gin"
)

// likeUseCaseFake 记录文章点赞和取消点赞调用。
type likeUseCaseFake struct {
	likedUserID     uint64 // likedUserID 是点赞用户标识。
	likedArticleID  uint64 // likedArticleID 是点赞文章标识。
	cancelUserID    uint64 // cancelUserID 是取消用户标识。
	cancelArticleID uint64 // cancelArticleID 是取消文章标识。
}

// LikeArticle 记录点赞调用。
func (f *likeUseCaseFake) LikeArticle(_ context.Context, userID, articleID uint64) error {
	// 1. 保存当前用户和文章标识
	f.likedUserID, f.likedArticleID = userID, articleID
	return nil
}

// CancelArticleLike 记录取消点赞调用。
func (f *likeUseCaseFake) CancelArticleLike(_ context.Context, userID, articleID uint64) error {
	// 1. 保存当前用户和文章标识
	f.cancelUserID, f.cancelArticleID = userID, articleID
	return nil
}

// TestLikeEndpointsUseCurrentAuthenticatedUser 验证点赞入口只使用认证上下文身份。
func TestLikeEndpointsUseCurrentAuthenticatedUser(t *testing.T) {
	// 1. 点赞和取消点赞都将当前用户与请求文章传入点赞上下文
	useCase := &likeUseCaseFake{}
	server := NewLikeServer(useCase).(*LikeService)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/auth/article/like", nil)
	identity.SetCurrentUser(ctx, identity.CurrentUser{ID: 7, Role: 1})
	if _, err := server.LikeArticle(ctx, &likeapi.ArticleLikeRequest{ArticleId: 9}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.CancelArticleLike(ctx, &likeapi.ArticleLikeRequest{ArticleId: 9}); err != nil {
		t.Fatal(err)
	}
	if useCase.likedUserID != 7 || useCase.likedArticleID != 9 || useCase.cancelUserID != 7 || useCase.cancelArticleID != 9 {
		t.Fatalf("use case=%#v", useCase)
	}
}

// TestLikeEndpointsRejectMissingIdentity 验证应用服务在缺少认证中间件时仍拒绝请求。
func TestLikeEndpointsRejectMissingIdentity(t *testing.T) {
	// 1. 未注入当前用户时不调用点赞领域服务
	useCase := &likeUseCaseFake{}
	server := NewLikeServer(useCase).(*LikeService)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/auth/article/like", nil)
	if _, err := server.LikeArticle(ctx, &likeapi.ArticleLikeRequest{ArticleId: 9}); err == nil {
		t.Fatal("missing identity was accepted")
	}
	if useCase.likedArticleID != 0 {
		t.Fatalf("unexpected like call=%#v", useCase)
	}
}
