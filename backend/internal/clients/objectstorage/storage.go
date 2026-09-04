// Package objectstorage 提供文章正文图片使用的 MinIO 适配器。
package objectstorage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// minioPresigner 定义生成 PUT 预签名地址所需的 MinIO SDK 能力。
type minioPresigner interface {
	// PresignedPutObject 生成指定对象的 PUT 预签名地址。
	PresignedPutObject(context.Context, string, string, time.Duration) (*url.URL, error)
}

// Storage 封装 MinIO PUT 预签名和公开对象地址拼接。
type Storage struct {
	client        minioPresigner // client 是 MinIO SDK 预签名客户端。
	bucket        string         // bucket 是正文图片存储桶。
	publicBaseURL string         // publicBaseURL 是图片公开访问域名。
}

// NewStorage 根据应用配置创建 MinIO 对象存储适配器。
func NewStorage(config *conf.Config) (*Storage, error) {
	// 1. 校验正文图片存储所需配置，避免启动后生成无效地址
	if config == nil || config.GetData() == nil {
		return nil, fmt.Errorf("缺少 MinIO 配置")
	}
	storageConfig := config.GetData().GetObjectStorage()
	if storageConfig == nil || storageConfig.GetEndpoint() == "" || storageConfig.GetAccessKey() == "" || storageConfig.GetSecretKey() == "" || storageConfig.GetBucket() == "" || storageConfig.GetPublicUrl() == "" {
		return nil, fmt.Errorf("缺少 MinIO endpoint、access_key、secret_key、bucket 或 public_url 配置")
	}

	// 2. 使用 MinIO SDK 和静态凭据创建预签名客户端
	client, err := minio.New(storageConfig.GetEndpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4(storageConfig.GetAccessKey(), storageConfig.GetSecretKey(), ""),
		Secure: storageConfig.GetUseSsl(),
	})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 客户端: %w", err)
	}
	return &Storage{client: client, bucket: storageConfig.GetBucket(), publicBaseURL: storageConfig.GetPublicUrl()}, nil
}

// PresignPut 生成带签名和指定有效期的 MinIO PUT 地址。
func (s *Storage) PresignPut(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	// 1. MinIO SDK 将有效期和签名参数写入预签名 URL
	uploadURL, err := s.client.PresignedPutObject(ctx, s.bucket, objectKey, expires)
	if err != nil {
		return "", err
	}
	return uploadURL.String(), nil
}

// PublicURL 将稳定对象键转换为公开图片地址。
func (s *Storage) PublicURL(objectKey string) string {
	// 1. 公开域名只参与读取地址拼接，不进入文章正文或图片记录
	return strings.TrimRight(s.publicBaseURL, "/") + "/" + strings.TrimLeft(objectKey, "/")
}

// ProvideAllowedImageExtensions 将配置白名单转换为文章领域值。
func ProvideAllowedImageExtensions(config *conf.Config) article.AllowedImageExtensions {
	// 1. 规范化扩展名并忽略空配置项
	allowed := make(article.AllowedImageExtensions)
	if config == nil || config.GetData().GetObjectStorage() == nil {
		return allowed
	}
	for _, extension := range config.GetData().GetObjectStorage().GetImageExtensions() {
		extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
		if extension != "" {
			allowed[extension] = struct{}{}
		}
	}
	return allowed
}
