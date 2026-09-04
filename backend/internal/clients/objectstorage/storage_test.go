package objectstorage

import (
	"context"
	"net/url"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

// fakePresigner 记录 MinIO SDK 预签名参数。
type fakePresigner struct {
	expires time.Duration // expires 是传入 MinIO SDK 的有效期。
}

// PresignedPutObject 返回带签名参数的测试 URL。
func (f *fakePresigner) PresignedPutObject(_ context.Context, _ string, _ string, expires time.Duration) (*url.URL, error) {
	// 1. 记录 SDK 收到的有效期
	f.expires = expires
	return url.Parse("https://minio.test/upload?X-Amz-Signature=test&X-Amz-Expires=600")
}

// TestStoragePresignPutUsesMinIOSignatureAndExpiration 验证适配器把十分钟有效期传给 MinIO SDK。
func TestStoragePresignPutUsesMinIOSignatureAndExpiration(t *testing.T) {
	// 1. 注入 MinIO SDK 接缝，避免测试依赖远端服务
	presigner := &fakePresigner{}
	storage := &Storage{client: presigner, bucket: "article-images"}
	rawURL, err := storage.PresignPut(context.Background(), "article/202609/image.png", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	// 2. 预签名 URL 必须保留签名参数和十分钟有效期
	if presigner.expires != 10*time.Minute || parsed.Query().Get("X-Amz-Signature") == "" || parsed.Query().Get("X-Amz-Expires") != "600" {
		t.Fatalf("url = %s, expires = %s", parsed, presigner.expires)
	}
}

// TestProvideAllowedImageExtensionsNormalizesConfiguration 验证图片扩展名白名单规范化。
func TestProvideAllowedImageExtensionsNormalizesConfiguration(t *testing.T) {
	// 1. 配置中的点号和大小写不会影响领域白名单
	allowed := ProvideAllowedImageExtensions(&conf.Config{Data: &conf.Data{ObjectStorage: &conf.ObjectStorage{
		ImageExtensions: []string{".JPG", "png"},
	}}})
	if _, ok := allowed["jpg"]; !ok {
		t.Fatalf("allowed = %#v", allowed)
	}
	if _, ok := allowed["png"]; !ok {
		t.Fatalf("allowed = %#v", allowed)
	}
}

// TestNewStorageRequiresPublicURL 验证公开图片地址配置不可缺失。
func TestNewStorageRequiresPublicURL(t *testing.T) {
	// 1. 缺失 public_url 时启动必须失败
	_, err := NewStorage(&conf.Config{Data: &conf.Data{ObjectStorage: &conf.ObjectStorage{
		Endpoint: "minio.example.test", AccessKey: "access", SecretKey: "secret", Bucket: "article-images",
	}}})
	if err == nil {
		t.Fatal("NewStorage() error = nil")
	}
}

// TestNewStorageRequiresCredentials 验证预签名所需的静态凭据不可缺失。
func TestNewStorageRequiresCredentials(t *testing.T) {
	// 1. 缺失访问凭据时启动必须失败，避免运行时才生成无效签名
	_, err := NewStorage(&conf.Config{Data: &conf.Data{ObjectStorage: &conf.ObjectStorage{
		Endpoint: "minio.example.test", Bucket: "article-images", PublicUrl: "https://cdn.example.test",
	}}})
	if err == nil {
		t.Fatal("NewStorage() error = nil")
	}
}
