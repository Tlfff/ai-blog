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
	images       map[uint64]*entity.Image // images 是可绑定图片集合。
	created      *entity.Article          // created 是已创建文章。
	updated      *entity.Article          // updated 是已更新文章。
	updatedIDs   []uint64                 // updatedIDs 是更新正文引用的图片标识。
	publishedID  uint64                   // publishedID 是已发布文章标识。
	published    *entity.Article          // published 是执行领域规则后的文章。
	current      *entity.Article          // current 是仓储锁定的当前文章。
	detailErr    error                    // detailErr 是详情查询预设错误。
	publicDetail *entity.Detail           // publicDetail 是公开文章详情。
	publicErr    error                    // publicErr 是公开详情预设错误。
	deleted      uint64                   // deleted 是被清理的图片标识。
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

// UpdateArticle 在测试文章上执行领域规则并记录图片关系。
func (f *fakeRepository) UpdateArticle(_ context.Context, articleID uint64, imageIDs []uint64, mutate ArticleMutation) error {
	// 1. 复制仓储锁定的当前文章并执行领域更新规则
	current := f.current
	if current == nil {
		current = &entity.Article{ID: articleID, AuthorID: 7, Status: StatusDraft}
	}
	updated := *current
	if err := mutate(&updated); err != nil {
		return err
	}

	// 2. 记录更新结果和正文图片关系
	f.updated = &updated
	f.updatedIDs = append([]uint64(nil), imageIDs...)
	return nil
}

// PublishArticle 在测试文章上执行领域发布规则。
func (f *fakeRepository) PublishArticle(_ context.Context, articleID uint64, mutate ArticleMutation) error {
	// 1. 复制仓储锁定的当前文章并执行领域发布规则
	current := f.current
	if current == nil {
		current = &entity.Article{ID: articleID, AuthorID: 7, Status: StatusDraft}
	}
	published := *current
	if err := mutate(&published); err != nil {
		return err
	}

	// 2. 记录文章发布结果
	f.publishedID = articleID
	f.published = &published
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

// FindPublicDetail 返回预设公开文章详情。
func (f *fakeRepository) FindPublicDetail(context.Context, uint64) (*entity.Detail, error) {
	// 1. 返回预设公开详情或错误
	if f.publicErr != nil {
		return nil, f.publicErr
	}
	if f.publicDetail != nil {
		return f.publicDetail, nil
	}
	return &entity.Detail{Article: &entity.Article{ID: 1, Status: StatusPublished}}, nil
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
	// 1. 定义正文图片不存在和已绑定的失败场景
	boundArticleID := uint64(9)
	tests := []struct {
		name    string                   // name 是场景名称。
		images  map[uint64]*entity.Image // images 是可查询图片。
		wantErr error                    // wantErr 是预期错误。
	}{
		{name: "图片不存在", images: map[uint64]*entity.Image{1: {ID: 1}}, wantErr: ErrImageNotFound},
		{name: "图片已绑定", images: map[uint64]*entity.Image{1: {ID: 1}, 2: {ID: 2, ArticleID: &boundArticleID}}, wantErr: ErrImageAlreadyBound},
	}
	// 2. 逐项验证任一图片不可用时不记录文章
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

// TestUpdateArticlePassesOnlyMarkdownImageReferences 验证更新将稳定图片引用交给原子仓储同步。
func TestUpdateArticlePassesOnlyMarkdownImageReferences(t *testing.T) {
	// 1. 更新正文同时包含系统图片、外部图片和普通占位文本
	repository := &fakeRepository{}
	service := newTestService(repository, &fakeStorage{}, &fakeLikeReader{}, &fakeGuard{acquired: true})
	err := service.Update(context.Background(), UpdateCommand{
		ArticleID: 9, AuthorID: 7, Title: "新标题",
		Content: "![系统图](image://2) ![外部图](https://example.test/a.png) `image://3`",
		Tags:    []string{"Go"}, Status: StatusDraft,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 领域服务只传递 Markdown 系统图片引用和更新字段
	if repository.updated == nil || repository.updated.ID != 9 || repository.updated.AuthorID != 7 ||
		repository.updated.Title != "新标题" || len(repository.updatedIDs) != 1 || repository.updatedIDs[0] != 2 {
		t.Fatalf("article = %#v, image IDs = %v", repository.updated, repository.updatedIDs)
	}
}

// TestUpdateArticleRejectsInvalidStatus 验证非法状态不会进入文章仓储。
func TestUpdateArticleRejectsInvalidStatus(t *testing.T) {
	// 1. 已删除状态不能作为更新接口的目标状态
	repository := &fakeRepository{}
	service := newTestService(repository, &fakeStorage{}, &fakeLikeReader{}, &fakeGuard{acquired: true})
	err := service.Update(context.Background(), UpdateCommand{ArticleID: 9, AuthorID: 7, Title: "标题", Content: "正文", Status: StatusDeleted})
	if !errors.Is(err, ErrInvalidStatus) || repository.updated != nil {
		t.Fatalf("error = %v, article = %#v", err, repository.updated)
	}
}

// TestPublishArticleAppliesPublishedStatus 验证发布规则写入状态和修改时间。
func TestPublishArticleAppliesPublishedStatus(t *testing.T) {
	// 1. 发布作者自己的草稿文章
	repository := &fakeRepository{}
	service := newTestService(repository, &fakeStorage{}, &fakeLikeReader{}, &fakeGuard{acquired: true})
	if err := service.Publish(context.Background(), 9, 7); err != nil {
		t.Fatal(err)
	}

	// 2. 事务内领域 mutation 必须写入发表状态和修改时间
	if repository.publishedID != 9 || repository.published == nil || repository.published.Status != StatusPublished || repository.published.UpdatedTime.IsZero() {
		t.Fatalf("article ID = %d, article = %#v", repository.publishedID, repository.published)
	}
}

// TestArticleMutationRejectsNonOwnerAndDeletedArticle 验证更新和发布的作者及删除状态规则。
func TestArticleMutationRejectsNonOwnerAndDeletedArticle(t *testing.T) {
	// 1. 定义非作者和已删除文章的领域场景
	tests := []struct {
		name      string          // name 是测试场景名称。
		current   *entity.Article // current 是仓储锁定的当前文章。
		wantError error           // wantError 是预期领域错误。
	}{
		{name: "非作者不能修改", current: &entity.Article{ID: 9, AuthorID: 8, Status: StatusDraft}, wantError: ErrArticleNotOwned},
		{name: "已删除文章不能修改", current: &entity.Article{ID: 9, AuthorID: 7, Status: StatusDeleted}, wantError: ErrArticleDeleted},
	}

	// 2. 逐项验证更新和发布复用相同领域规则
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeRepository{current: test.current}
			service := newTestService(repository, &fakeStorage{}, &fakeLikeReader{}, &fakeGuard{acquired: true})
			command := UpdateCommand{ArticleID: 9, AuthorID: 7, Title: "标题", Content: "正文", Status: StatusDraft}
			if err := service.Update(context.Background(), command); !errors.Is(err, test.wantError) {
				t.Fatalf("Update() error = %v, want %v", err, test.wantError)
			}
			if err := service.Publish(context.Background(), 9, 7); !errors.Is(err, test.wantError) {
				t.Fatalf("Publish() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

// TestPublicDetailUsesGuestAndLoggedInLikeState 验证游客与登录用户的公开点赞状态。
func TestPublicDetailUsesGuestAndLoggedInLikeState(t *testing.T) {
	// 1. 游客查询由点赞契约返回未点赞
	repository := &fakeRepository{publicDetail: &entity.Detail{Article: &entity.Article{ID: 9, Status: StatusPublished}}}
	guestService := newTestService(repository, &fakeStorage{}, &fakeLikeReader{liked: false}, &fakeGuard{acquired: true})
	guestDetail, err := guestService.PublicDetail(context.Background(), 9, 0)
	if err != nil || guestDetail.IsLiked {
		t.Fatalf("detail = %#v, error = %v", guestDetail, err)
	}

	// 2. 登录用户查询返回实际点赞状态
	userService := newTestService(repository, &fakeStorage{}, &fakeLikeReader{liked: true}, &fakeGuard{acquired: true})
	userDetail, err := userService.PublicDetail(context.Background(), 9, 7)
	if err != nil || !userDetail.IsLiked {
		t.Fatalf("detail = %#v, error = %v", userDetail, err)
	}
}
