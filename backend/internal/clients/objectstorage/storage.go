// Package objectstorage 提供文章正文图片使用的 MinIO 适配器。
package objectstorage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	article "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// minioClient 定义正文图片预签名和对象删除所需的 MinIO SDK 能力。
type minioClient interface {
	// PresignedPutObject 生成指定对象的 PUT 预签名地址。
	PresignedPutObject(context.Context, string, string, time.Duration) (*url.URL, error)
	// RemoveObject 删除指定存储桶中的对象。
	RemoveObject(context.Context, string, string, minio.RemoveObjectOptions) error
	// CopyObject 在同一存储桶内复制对象，用于可回滚删除。
	CopyObject(context.Context, minio.CopyDestOptions, minio.CopySrcOptions) (minio.UploadInfo, error)
	// ListObjects 按前缀列出用于崩溃恢复的隔离对象。
	ListObjects(context.Context, string, minio.ListObjectsOptions) <-chan minio.ObjectInfo
}

const (
	stagedDeletionPrefix      = ".article-trash/"      // stagedDeletionPrefix 是不可公开访问的对象删除隔离前缀。
	stagedDeletionGracePeriod = 5 * time.Minute        // stagedDeletionGracePeriod 避免恢复其他实例仍在处理的暂存对象。
	stagedDeletionRollbackTTL = 30 * time.Second       // stagedDeletionRollbackTTL 是单次对象补偿的最长时间。
	minioOperationAttempts    = 3                      // minioOperationAttempts 是对象复制或删除的最大尝试次数。
	minioRetryDelay           = 100 * time.Millisecond // minioRetryDelay 是 MinIO 重试的初始间隔。
)

// Storage 封装 MinIO PUT 预签名和公开对象地址拼接。
type Storage struct {
	client        minioClient // client 是 MinIO SDK 正文图片客户端。
	bucket        string      // bucket 是正文图片存储桶。
	publicBaseURL string      // publicBaseURL 是图片公开访问域名。
}

// stagedObjectDeletion 表示原对象已删除但隔离副本仍可恢复的操作。
type stagedObjectDeletion struct {
	storage     *Storage // storage 是执行复制和删除的对象存储适配器。
	originalKey string   // originalKey 是数据库保存的稳定对象键。
	stagedKey   string   // stagedKey 是仅用于提交或回滚的隔离对象键。
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

// StageDelete 暂存并删除原始对象，返回可提交或回滚的删除操作。
func (s *Storage) StageDelete(ctx context.Context, objectKey string) (article.StagedObjectDeletion, error) {
	// 1. 将原始对象复制到不会被公开引用的唯一隔离键
	deletion := &stagedObjectDeletion{
		storage: s, originalKey: objectKey, stagedKey: stagedObjectKey(objectKey),
	}
	if err := deletion.storage.copyObject(ctx, deletion.originalKey, deletion.stagedKey); err != nil {
		return nil, fmt.Errorf("复制正文图片到隔离对象: %w", err)
	}

	// 2. 删除原始对象；失败时用独立上下文恢复并清理隔离副本
	if err := retryMinIOOperation(ctx, func() error {
		return s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
	}); err != nil {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stagedDeletionRollbackTTL)
		defer cancel()
		if rollbackErr := deletion.Rollback(rollbackCtx); rollbackErr != nil {
			return nil, fmt.Errorf("删除正文图片原始对象: %w；恢复对象: %v", err, rollbackErr)
		}
		return nil, fmt.Errorf("删除正文图片原始对象: %w", err)
	}
	return deletion, nil
}

// ListStagedDeletions 列出超过安全宽限期的持久化暂存删除记录。
func (s *Storage) ListStagedDeletions(ctx context.Context) ([]article.StagedObjectDeletion, error) {
	// 1. 只扫描隔离前缀，并跳过其他实例可能仍在处理的新对象
	cutoff := time.Now().Add(-stagedDeletionGracePeriod)
	deletions := make([]article.StagedObjectDeletion, 0)
	objects := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: stagedDeletionPrefix, Recursive: true})
	for object := range objects {
		if object.Err != nil {
			return nil, object.Err
		}
		if object.LastModified.After(cutoff) {
			continue
		}
		originalKey, err := originalObjectKey(object.Key)
		if err != nil {
			return nil, err
		}
		deletions = append(deletions, &stagedObjectDeletion{storage: s, originalKey: originalKey, stagedKey: object.Key})
	}
	return deletions, nil
}

// OriginalKey 返回数据库保存的原始稳定对象键。
func (d *stagedObjectDeletion) OriginalKey() string {
	// 1. 暴露恢复决策所需的稳定对象键
	return d.originalKey
}

// Commit 清理隔离副本，完成对象删除。
func (d *stagedObjectDeletion) Commit(ctx context.Context) error {
	// 1. 数据库记录已提交删除后移除隔离对象
	if err := retryMinIOOperation(ctx, func() error {
		return d.storage.client.RemoveObject(ctx, d.storage.bucket, d.stagedKey, minio.RemoveObjectOptions{})
	}); err != nil {
		return fmt.Errorf("清理正文图片隔离对象 %q: %w", d.stagedKey, err)
	}
	return nil
}

// Rollback 将隔离副本恢复到原始稳定对象键。
func (d *stagedObjectDeletion) Rollback(ctx context.Context) error {
	// 1. 先恢复原始对象，确保数据库引用重新可用
	if err := d.storage.copyObject(ctx, d.stagedKey, d.originalKey); err != nil {
		return fmt.Errorf("恢复正文图片原始对象 %q: %w", d.originalKey, err)
	}

	// 2. 原始对象恢复后清理隔离副本
	if err := retryMinIOOperation(ctx, func() error {
		return d.storage.client.RemoveObject(ctx, d.storage.bucket, d.stagedKey, minio.RemoveObjectOptions{})
	}); err != nil {
		return fmt.Errorf("清理已恢复的正文图片隔离对象 %q: %w", d.stagedKey, err)
	}
	return nil
}

// copyObject 在正文图片存储桶内复制对象。
func (s *Storage) copyObject(ctx context.Context, sourceKey, destinationKey string) error {
	// 1. 使用 MinIO 服务端复制，避免图片内容经过业务进程
	return retryMinIOOperation(ctx, func() error {
		_, err := s.client.CopyObject(ctx,
			minio.CopyDestOptions{Bucket: s.bucket, Object: destinationKey},
			minio.CopySrcOptions{Bucket: s.bucket, Object: sourceKey},
		)
		return err
	})
}

// stagedObjectKey 将原始对象键编码为可持久化恢复的隔离键。
func stagedObjectKey(originalKey string) string {
	// 1. Base64 URL 编码保留完整原对象键且避免路径字符歧义
	return stagedDeletionPrefix + base64.RawURLEncoding.EncodeToString([]byte(originalKey))
}

// originalObjectKey 从隔离对象键恢复数据库使用的稳定对象键。
func originalObjectKey(stagedKey string) (string, error) {
	// 1. 隔离键必须使用当前可逆编码格式
	if !strings.HasPrefix(stagedKey, stagedDeletionPrefix) {
		return "", fmt.Errorf("无效正文图片隔离对象键 %q", stagedKey)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(stagedKey, stagedDeletionPrefix))
	if err != nil {
		return "", fmt.Errorf("解析正文图片隔离对象键 %q: %w", stagedKey, err)
	}
	if len(decoded) == 0 {
		return "", fmt.Errorf("解析正文图片隔离对象键 %q: 原始对象键为空", stagedKey)
	}
	return string(decoded), nil
}

// retryMinIOOperation 使用有界退避重试短暂的 MinIO 操作失败。
func retryMinIOOperation(ctx context.Context, operation func() error) error {
	// 1. 首次立即执行，后续失败按指数间隔重试
	var err error
	delay := minioRetryDelay
	for attempt := 0; attempt < minioOperationAttempts; attempt++ {
		if err = operation(); err == nil {
			return nil
		}
		if attempt == minioOperationAttempts-1 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
		delay *= 2
	}
	return err
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
