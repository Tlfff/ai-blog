package article

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
)

// fakeRepository 记录文章领域服务的持久化调用。
type fakeRepository struct {
	images    map[uint64]*entity.Image // images 是可绑定图片集合。
	created   *entity.Article          // created 是已创建文章。
	detailErr error                    // detailErr 是详情查询预设错误。
	deleted   uint64                   // deleted 是被清理的图片标识。
}

// CreatePendingImage 创建测试图片记录。
func (f *fakeRepository) CreatePendingImage(_ context.Context, image *entity.Image) error {
	// 1. 回写固定图片标识
	image.ID = 8
	return nil
}

// DeletePendingImage 记录预签名失败后的图片清理。
func (f *fakeRepository) DeletePendingImage(_ context.Context, imageID uint64) error {
	// 1. 记录领域服务请求清理的图片标识
	f.deleted = imageID
	return nil
}

// CreateArticle 模拟事务绑定的失败和成功结果。
func (f *fakeRepository) CreateArticle(_ context.Context, article *entity.Article, imageIDs []uint64) error {
	// 1. 任一图片不可用时保持文章未创建
	for _, imageID := range imageIDs {
		image := f.images[imageID]
		if image == nil {
			return ErrImageNotFound
		}
		if image.ArticleID != nil {
			return ErrImageAlreadyBound
		}
	}
	// 2. 全部图片可用时记录文章
	f.created = article
	return nil
}

// FindDetail 返回带互动统计的测试详情。
func (f *fakeRepository) FindDetail(context.Context, uint64, uint64) (*entity.Detail, error) {
	// 1. 返回预设归属错误或固定详情
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	return &entity.Detail{Article: &entity.Article{ID: 1, LikeCount: 4}}, nil
}

// fakeStorage 记录预签名有效期。
type fakeStorage struct {
	expires time.Duration // expires 是预签名地址有效期。
	err     error         // err 是预签名失败。
}

// PresignPut 返回测试预签名地址。
func (f *fakeStorage) PresignPut(_ context.Context, _ string, expires time.Duration) (string, error) {
	// 1. 记录有效期并返回测试地址
	f.expires = expires
	if f.err != nil {
		return "", f.err
	}
	return "https://upload.test", nil
}

// PublicURL 返回测试公开地址。
func (*fakeStorage) PublicURL(string) string {
	// 1. 返回固定地址
	return "https://cdn.test/image"
}

// fakeLikeReader 返回预设点赞状态。
type fakeLikeReader struct {
	liked bool // liked 是当前用户点赞状态。
}

// IsArticleLiked 返回当前用户点赞状态。
func (f *fakeLikeReader) IsArticleLiked(context.Context, uint64, uint64) (bool, error) {
	// 1. 返回预设结果
	return f.liked, nil
}

// fakeGuard 模拟 Redis 防重复键。
type fakeGuard struct {
	acquired bool          // acquired 表示是否成功占用。
	ttl      time.Duration // ttl 是防重键有效期。
}

// Acquire 返回预设占用结果。
func (f *fakeGuard) Acquire(_ context.Context, _ string, ttl time.Duration) (bool, error) {
	// 1. 记录 TTL 并返回占用结果
	f.ttl = ttl
	return f.acquired, nil
}

// newTestService 创建文章领域服务测试夹具。
func newTestService(repository Repository, storage Storage, likes LikeReader, guard SubmissionGuard) *Service {
	// 1. 使用固定图片扩展名白名单
	return NewService(repository, storage, likes, guard, AllowedImageExtensions{"jpg": {}, "png": {}})
}

// TestUploadImageValidatesExtensionAndExpiration 验证扩展名白名单及十分钟有效期。
func TestUploadImageValidatesExtensionAndExpiration(t *testing.T) {
	// 1. 非白名单扩展名必须失败
	storage := &fakeStorage{}
	service := newTestService(&fakeRepository{}, storage, &fakeLikeReader{}, &fakeGuard{acquired: true})
	if _, err := service.UploadImage(context.Background(), 7, "exe"); !errors.Is(err, ErrInvalidImageExtension) {
		t.Fatalf("UploadImage() error = %v", err)
	}

	// 2. 白名单扩展名生成十分钟有效凭证
	result, err := service.UploadImage(context.Background(), 7, ".PNG")
	if err != nil {
		t.Fatal(err)
	}
	if result.ImageID != 8 || storage.expires != 10*time.Minute {
		t.Fatalf("result = %#v, expires = %s", result, storage.expires)
	}
}

// TestCreateArticleRejectsUnavailableImages 验证图片不可用时文章整体不创建。
func TestCreateArticleRejectsUnavailableImages(t *testing.T) {
	boundArticleID := uint64(9)
	tests := []struct {
		name    string                   // name 是场景名称。
		images  map[uint64]*entity.Image // images 是可查询图片。
		wantErr error                    // wantErr 是预期错误。
	}{
		{name: "图片不存在", images: map[uint64]*entity.Image{1: {ID: 1}}, wantErr: ErrImageNotFound},
		{name: "图片已绑定", images: map[uint64]*entity.Image{1: {ID: 1}, 2: {ID: 2, ArticleID: &boundArticleID}}, wantErr: ErrImageAlreadyBound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 1. 创建请求同时引用两张图片
			repository := &fakeRepository{images: test.images}
			service := newTestService(repository, &fakeStorage{}, &fakeLikeReader{}, &fakeGuard{acquired: true})
			err := service.Create(context.Background(), CreateCommand{AuthorID: 7, Title: "标题", Content: "![图一](image://1) ![图二](image://2)", Status: StatusDraft})
			if !errors.Is(err, test.wantErr) || repository.created != nil {
				t.Fatalf("error = %v, article = %#v", err, repository.created)
			}
		})
	}
}

// TestReferencedImageIDsOnlyReadsMarkdownImages 验证只绑定 Markdown 图片节点中的稳定引用。
func TestReferencedImageIDsOnlyReadsMarkdownImages(t *testing.T) {
	// 1. 普通文本、代码和外部地址中的 image:// 文本不能被当作正文图片
	content := "普通文本 image://1\n\n`image://2`\n\n```text\nimage://3\n```\n\n![正文图](image://4)\n\n![重复图](image://4)\n\n![外部图](https://example.test/image://5)"
	imageIDs := referencedImageIDs(content)
	if len(imageIDs) != 1 || imageIDs[0] != 4 {
		t.Fatalf("referencedImageIDs() = %v, want [4]", imageIDs)
	}
}

// TestCreateArticleUsesSharedTwoSecondGuard 验证共享两秒防重策略。
func TestCreateArticleUsesSharedTwoSecondGuard(t *testing.T) {
	// 1. 防重键已存在时拒绝调用文章仓储
	guard := &fakeGuard{acquired: false}
	repository := &fakeRepository{}
	service := newTestService(repository, &fakeStorage{}, &fakeLikeReader{}, guard)
	err := service.Create(context.Background(), CreateCommand{AuthorID: 7, Title: "标题", Content: "正文", Status: StatusDraft})
	if !errors.Is(err, ErrDuplicateSubmission) || repository.created != nil || guard.ttl != 2*time.Second {
		t.Fatalf("error = %v, article = %#v, ttl = %s", err, repository.created, guard.ttl)
	}
}

// TestDetailIncludesCurrentLikeState 验证详情补充当前点赞状态。
func TestDetailIncludesCurrentLikeState(t *testing.T) {
	// 1. 点赞查询结果和点赞数投影均写入详情
	service := newTestService(&fakeRepository{}, &fakeStorage{}, &fakeLikeReader{liked: true}, &fakeGuard{acquired: true})
	detail, err := service.Detail(context.Background(), 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.IsLiked || detail.Article.LikeCount != 4 {
		t.Fatalf("detail = %#v", detail)
	}
}

// TestDetailRejectsNonOwner 验证非作者管理员不能读取 /me 详情。
func TestDetailRejectsNonOwner(t *testing.T) {
	// 1. 文章上下文返回归属校验错误
	service := newTestService(&fakeRepository{detailErr: ErrArticleNotOwned}, &fakeStorage{}, &fakeLikeReader{}, &fakeGuard{acquired: true})
	if _, err := service.Detail(context.Background(), 1, 8); !errors.Is(err, ErrArticleNotOwned) {
		t.Fatalf("Detail() error = %v", err)
	}
}

// TestUploadImageCleansRecordOnPresignFailure 验证预签名失败时清理未绑定图片记录。
func TestUploadImageCleansRecordOnPresignFailure(t *testing.T) {
	// 1. MinIO 预签名失败，领域服务必须删除刚创建的图片记录
	repository := &fakeRepository{}
	service := newTestService(repository, &fakeStorage{err: errors.New("minio unavailable")}, &fakeLikeReader{}, &fakeGuard{acquired: true})
	if _, err := service.UploadImage(context.Background(), 7, "png"); err == nil || repository.deleted != 8 {
		t.Fatalf("error = %v, deleted = %d", err, repository.deleted)
	}
}
