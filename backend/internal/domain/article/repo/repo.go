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
	"xorm.io/xorm/schemas"
)

// transactionClient 是文章写入必须具备的 MySQL 事务能力。
type transactionClient interface {
	// Transaction 在同一数据库事务中执行文章与图片写入。
	Transaction(func(*xorm.Session) (interface{}, error)) (interface{}, error)
}

// Repository 使用 MySQL 实现文章领域仓储。
type Repository struct {
	client      clients.MysqlClient // client 提供普通文章查询和写入能力。
	transaction transactionClient   // transaction 提供不可降级的文章写入和图片关系事务。
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
			if err := forUpdate(session.In("id", imageIDs)).Find(&images); err != nil {
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

// UpdateArticle 在同一事务中执行领域更新并同步正文图片绑定关系。
func (r *Repository) UpdateArticle(ctx context.Context, articleID uint64, imageIDs []uint64, mutate article.ArticleMutation) error {
	// 1. 锁定文章和相关图片，确保领域决策、更新、绑定及解绑原子完成
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		currentArticle, err := findArticleForUpdate(session, articleID)
		if err != nil {
			return nil, err
		}
		articleEntity := factory.ArticleFromPO(currentArticle)
		if err := mutate(articleEntity); err != nil {
			return nil, err
		}

		// 1.1 锁定更新后正文引用的全部图片并拒绝不存在或归属其他文章的记录
		newImageIDs, err := availableImageIDs(session, articleEntity.ID, imageIDs)
		if err != nil {
			return nil, err
		}

		// 1.2 锁定当前图片关系并找出需要解绑的图片
		currentImages := make([]*po.Image, 0)
		if err := forUpdate(session.Where("article_id = ?", articleEntity.ID)).Find(&currentImages); err != nil {
			return nil, err
		}
		desiredImages := make(map[uint64]struct{}, len(imageIDs))
		for _, imageID := range imageIDs {
			desiredImages[imageID] = struct{}{}
		}
		removedImageIDs := make([]uint64, 0)
		for _, image := range currentImages {
			if _, retained := desiredImages[image.ID]; !retained {
				removedImageIDs = append(removedImageIDs, image.ID)
			}
		}

		// 1.3 更新文章主体，显式写入 updated_time 以触发后续 Binlog 同步
		articlePO := factory.ArticleToPO(articleEntity)
		if _, err := session.ID(articleEntity.ID).
			Cols("title", "content", "tags", "status", "updated_time").
			Update(articlePO); err != nil {
			return nil, err
		}

		// 1.4 只绑定原先未归属文章的新增图片
		if len(newImageIDs) > 0 {
			rows, err := session.In("id", newImageIDs).
				And("article_id IS NULL").
				Cols("article_id").
				Update(&po.Image{ArticleID: &articleEntity.ID})
			if err != nil {
				return nil, err
			}
			if rows != int64(len(newImageIDs)) {
				return nil, article.ErrImageAlreadyBound
			}
		}

		// 1.5 只解除正文已移除图片的文章归属，不删除对象和图片记录
		if len(removedImageIDs) > 0 {
			if _, err := session.In("id", removedImageIDs).
				And("article_id = ?", articleEntity.ID).
				Cols("article_id").
				Update(&po.Image{ArticleID: nil}); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

// ChangeArticleStatus 在同一事务中执行文章状态变更领域规则。
func (r *Repository) ChangeArticleStatus(ctx context.Context, articleID uint64, mutate article.ArticleMutation) error {
	// 1. 锁定文章，在事务内执行领域规则后保存状态字段
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		currentArticle, err := findArticleForUpdate(session, articleID)
		if err != nil {
			return nil, err
		}
		articleEntity := factory.ArticleFromPO(currentArticle)
		if err := mutate(articleEntity); err != nil {
			return nil, err
		}

		// 2. 显式更新状态和修改时间，由 articles 表 Binlog 驱动搜索同步
		_, err = session.ID(articleID).
			Cols("status", "updated_time").
			Update(&po.Article{Status: articleEntity.Status, UpdatedTime: articleEntity.UpdatedTime})
		return nil, err
	})
	return err
}

// ListArticles 分页查询当前作者符合状态筛选的文章。
func (r *Repository) ListArticles(ctx context.Context, query article.ListQuery) (*article.ListResult, error) {
	// 1. 统计当前作者符合状态筛选的文章总数
	total, err := r.listSession(ctx, query).Count(new(po.Article))
	if err != nil {
		return nil, err
	}

	// 2. 游标存在时优先使用游标，否则使用页码计算 Offset
	session := r.listSession(ctx, query)
	order := "id ASC"
	if query.IsDesc {
		order = "id DESC"
	}
	if query.LastID > 0 {
		operator := ">"
		if query.IsDesc {
			operator = "<"
		}
		session = session.And("id "+operator+" ?", query.LastID)
	} else {
		offset := int((query.Page - 1) * query.PageSize)
		session = session.Limit(int(query.PageSize), offset)
	}
	if query.LastID > 0 {
		session = session.Limit(int(query.PageSize))
	}

	// 3. 按文章标识稳定排序并转换为领域列表
	articlePOs := make([]*po.Article, 0, query.PageSize)
	if err := session.OrderBy(order).Find(&articlePOs); err != nil {
		return nil, err
	}
	articles := make([]*entity.Article, 0, len(articlePOs))
	for _, articlePO := range articlePOs {
		articles = append(articles, factory.ArticleFromPO(articlePO))
	}
	lastID := uint64(0)
	if len(articles) > 0 {
		lastID = articles[len(articles)-1].ID
	}
	return &article.ListResult{Articles: articles, LastID: lastID, Total: uint64(total), Page: query.Page, PageSize: query.PageSize}, nil
}

// FindClearTarget 查询待彻底删除的文章和全部绑定图片快照。
func (r *Repository) FindClearTarget(ctx context.Context, articleID uint64) (*article.ClearTarget, error) {
	// 1. 查询文章当前状态和作者信息
	articlePO := new(po.Article)
	found, err := r.client.Context(ctx).ID(articleID).Get(articlePO)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, article.ErrArticleNotFound
	}

	// 2. 查询当前绑定图片，供领域服务暂存对象和后续事务复核
	imagePOs := make([]*po.Image, 0)
	if err := r.client.Context(ctx).Where("article_id = ?", articleID).Find(&imagePOs); err != nil {
		return nil, err
	}
	return &article.ClearTarget{Article: factory.ArticleFromPO(articlePO), Images: imageEntities(imagePOs)}, nil
}

// ClearArticle 在同一事务中复核文章快照并硬删除数据库记录。
func (r *Repository) ClearArticle(ctx context.Context, articleID uint64, validate article.ArticleClearValidation) error {
	// 1. 锁定文章和全部绑定图片，避免数据库清理期间关系发生变化
	_, err := r.transaction.Transaction(func(session *xorm.Session) (interface{}, error) {
		session = session.Context(ctx)
		articlePO, err := findArticleForUpdate(session, articleID)
		if err != nil {
			return nil, err
		}
		imagePOs := make([]*po.Image, 0)
		if err := forUpdate(session.Where("article_id = ?", articleID)).Find(&imagePOs); err != nil {
			return nil, err
		}

		// 2. 在事务内重新执行领域权限、状态和图片快照校验
		if err := validate(factory.ArticleFromPO(articlePO), imageEntities(imagePOs)); err != nil {
			return nil, err
		}

		// 3. 对象已在事务外可回滚暂存，当前事务只删除图片和文章记录
		if _, err := session.Where("article_id = ?", articleID).Delete(new(po.Image)); err != nil {
			return nil, err
		}
		if _, err := session.ID(articleID).Delete(new(po.Article)); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

// imageEntities 将图片持久化对象转换为领域快照。
func imageEntities(imagePOs []*po.Image) []*entity.Image {
	// 1. 只复制彻底删除和详情映射需要的稳定字段
	images := make([]*entity.Image, 0, len(imagePOs))
	for _, imagePO := range imagePOs {
		images = append(images, &entity.Image{ID: imagePO.ID, ArticleID: imagePO.ArticleID, ObjectKey: imagePO.ObjectKey})
	}
	return images
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

	return r.detailFromArticle(ctx, articlePO)
}

// FindPublicDetail 查询已发表文章、作者公开字段和全部绑定图片。
func (r *Repository) FindPublicDetail(ctx context.Context, articleID uint64) (*entity.Detail, error) {
	// 1. 查询文章并拒绝公开读取草稿和已删除状态
	articlePO := new(po.Article)
	found, err := r.client.Context(ctx).Where("id = ?", articleID).Get(articlePO)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, article.ErrArticleNotFound
	}
	if articlePO.Status != article.StatusPublished {
		return nil, article.ErrArticleNotPublished
	}
	return r.detailFromArticle(ctx, articlePO)
}

// listSession 创建当前作者和状态筛选对应的查询会话。
func (r *Repository) listSession(ctx context.Context, query article.ListQuery) *xorm.Session {
	// 1. 后台文章管理始终只查询当前作者的数据
	session := r.client.Context(ctx).Where("author_id = ?", query.AuthorID)

	// 2. 根据兼容状态值追加全部、非删除或精确状态条件
	switch query.Status {
	case article.StatusAll:
	case article.StatusNotDeleted:
		session = session.And("status <> ?", article.StatusDeleted)
	default:
		session = session.And("status = ?", query.Status)
	}
	return session
}

// detailFromArticle 查询文章对应的作者公开字段和图片映射。
func (r *Repository) detailFromArticle(ctx context.Context, articlePO *po.Article) (*entity.Detail, error) {
	// 1. 查询作者公开展示字段
	authorPO := new(po.User)
	if _, err := r.client.Context(ctx).Where("id = ?", articlePO.AuthorID).Get(authorPO); err != nil {
		return nil, err
	}
	detail := &entity.Detail{
		Article: factory.ArticleFromPO(articlePO), AuthorNickname: authorPO.Nickname,
		AuthorAvatar: authorPO.Avatar, AuthorIP: authorPO.LastLoginIP,
	}

	// 2. 查询文章已绑定图片并转换为稳定映射
	images := make([]*po.Image, 0)
	if err := r.client.Context(ctx).Where("article_id = ?", articlePO.ID).Find(&images); err != nil {
		return nil, err
	}
	for _, image := range images {
		detail.Images = append(detail.Images, &entity.Image{ID: image.ID, ArticleID: image.ArticleID, ObjectKey: image.ObjectKey})
	}
	return detail, nil
}

// findArticleForUpdate 锁定并查询待修改文章。
func findArticleForUpdate(session *xorm.Session, articleID uint64) (*po.Article, error) {
	// 1. 使用主键锁定文章，避免并发更新绕过状态和作者校验
	articlePO := new(po.Article)
	found, err := forUpdate(session.ID(articleID)).Get(articlePO)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, article.ErrArticleNotFound
	}
	return articlePO, nil
}

// availableImageIDs 锁定目标图片并返回本次需要新绑定的图片标识。
func availableImageIDs(session *xorm.Session, articleID uint64, imageIDs []uint64) ([]uint64, error) {
	// 1. 空正文图片集合不需要查询或绑定
	if len(imageIDs) == 0 {
		return nil, nil
	}

	// 2. 锁定全部目标图片，避免并发文章同时绑定同一图片
	images := make([]*po.Image, 0, len(imageIDs))
	if err := forUpdate(session.In("id", imageIDs)).Find(&images); err != nil {
		return nil, err
	}
	if len(images) != len(imageIDs) {
		return nil, article.ErrImageNotFound
	}

	// 3. 已归属当前文章的图片保持绑定，未绑定图片交给更新语句新增关系
	newImageIDs := make([]uint64, 0, len(images))
	for _, image := range images {
		if image.ArticleID == nil {
			newImageIDs = append(newImageIDs, image.ID)
			continue
		}
		if *image.ArticleID != articleID {
			return nil, article.ErrImageAlreadyBound
		}
	}
	return newImageIDs, nil
}

// forUpdate 在支持行锁的生产 MySQL 上启用 FOR UPDATE。
func forUpdate(session *xorm.Session) *xorm.Session {
	// 1. SQLite 测试事务依赖数据库级写锁，生产 MySQL 使用显式行锁
	if session.Engine().Dialect().URI().DBType == schemas.MYSQL {
		return session.ForUpdate()
	}
	return session
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
