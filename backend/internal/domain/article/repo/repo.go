package repo

import (
	"context"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo/factory"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo/po"
	"xorm.io/xorm"
)

// transactionClient 是文章创建必须具备的 MySQL 事务能力。
type transactionClient interface {
	// Transaction 在同一数据库事务中执行文章与图片写入。
	Transaction(func(*xorm.Session) (interface{}, error)) (interface{}, error)
}

// Repository 使用 MySQL 实现文章领域仓储。
type Repository struct {
	client      clients.MysqlClient // client 提供普通文章查询和写入能力。
	transaction transactionClient   // transaction 提供不可降级的文章图片绑定事务。
}

// NewRepository 创建文章 MySQL 仓储。
func NewRepository(client clients.MysqlClient, transaction transactionClient) *Repository {
	// 1. 启动阶段拒绝缺少数据库或事务能力的客户端
	if client == nil {
		panic("文章仓储缺少 MySQL 客户端")
	}
	if transaction == nil {
		panic("文章仓储要求 MySQL 客户端支持事务")
	}
	return &Repository{client: client, transaction: transaction}
}

// CreatePendingImage 创建尚未归属文章的正文图片记录。
func (r *Repository) CreatePendingImage(ctx context.Context, image *entity.Image) error {
	// 1. 只保存稳定对象键，预签名地址不进入数据库
	imagePO := &po.Image{ObjectKey: image.ObjectKey, CreatedTime: time.Now()}
	if _, err := r.client.Context(ctx).Insert(imagePO); err != nil {
		return err
	}
	image.ID = imagePO.ID
	return nil
}

// DeletePendingImage 删除仍未绑定文章的正文图片记录。
func (r *Repository) DeletePendingImage(ctx context.Context, imageID uint64) error {
	// 1. 仅删除未绑定图片，避免误删已归属文章的图片
	_, err := r.client.Context(ctx).Where("id = ? AND article_id IS NULL", imageID).Delete(new(po.Image))
	return err
}

// CreateArticle 在同一事务中创建文章并并发安全地绑定全部图片。
func (r *Repository) CreateArticle(ctx context.Context, articleEntity *entity.Article, imageIDs []uint64) error {
	// 1. 所有检查、文章插入和图片绑定均在不可降级的事务中执行
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)

		// 1.1 锁定图片行，确保并发创建请求不能同时通过归属检查
		if len(imageIDs) > 0 {
			images := make([]*po.Image, 0, len(imageIDs))
			if err := session.In("id", imageIDs).ForUpdate().Find(&images); err != nil {
				return nil, err
			}
			if len(images) != len(imageIDs) {
				return nil, article.ErrImageNotFound
			}
			for _, image := range images {
				if image.ArticleID != nil {
					return nil, article.ErrImageAlreadyBound
				}
			}
		}

		// 1.2 图片全部可用后插入文章，后续失败会回滚该记录
		articlePO := factory.ArticleToPO(articleEntity)
		if _, err := session.Insert(articlePO); err != nil {
			return nil, err
		}
		articleEntity.ID = articlePO.ID

		// 1.3 绑定时再次限定 article_id IS NULL，并校验实际更新行数
		if len(imageIDs) > 0 {
			rows, err := session.In("id", imageIDs).
				And("article_id IS NULL").
				Cols("article_id").
				Update(&po.Image{ArticleID: &articleEntity.ID})
			if err != nil {
				return nil, err
			}
			if rows != int64(len(imageIDs)) {
				return nil, article.ErrImageAlreadyBound
			}
		}
		return nil, nil
	})
	return err
}

// FindDetail 查询非删除文章、作者公开字段和全部绑定图片。
func (r *Repository) FindDetail(ctx context.Context, articleID, userID uint64) (*entity.Detail, error) {
	// 1. 查询非删除文章及当前互动计数投影
	articlePO := new(po.Article)
	found, err := r.client.Context(ctx).
		Where("id = ? AND status <> ?", articleID, article.StatusDeleted).
		Get(articlePO)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, article.ErrArticleNotFound
	}
	if articlePO.AuthorID != userID {
		return nil, article.ErrArticleNotOwned
	}

	// 2. 查询作者公开展示字段
	authorPO := new(po.User)
	if _, err := r.client.Context(ctx).Where("id = ?", articlePO.AuthorID).Get(authorPO); err != nil {
		return nil, err
	}
	detail := &entity.Detail{
		Article: factory.ArticleFromPO(articlePO), AuthorNickname: authorPO.Nickname,
		AuthorAvatar: authorPO.Avatar, AuthorIP: authorPO.LastLoginIP,
	}

	// 3. 查询文章已绑定图片并转换为稳定映射
	images := make([]*po.Image, 0)
	if err := r.client.Context(ctx).Where("article_id = ?", articleID).Find(&images); err != nil {
		return nil, err
	}
	for _, image := range images {
		detail.Images = append(detail.Images, &entity.Image{ID: image.ID, ArticleID: image.ArticleID, ObjectKey: image.ObjectKey})
	}
	return detail, nil
}

// ProvideTransactionClient 将 MySQL 客户端显式暴露为文章事务契约。
func ProvideTransactionClient(client clients.MysqlClient) transactionClient {
	// 1. 事务契约不可缺失，避免文章创建降级为非事务写入
	transaction, ok := client.(transactionClient)
	if !ok {
		panic("文章仓储要求 MySQL 客户端支持事务")
	}
	return transaction
}
