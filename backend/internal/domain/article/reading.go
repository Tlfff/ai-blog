package article

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/entity"
	"github.com/google/uuid"
)

const (
	defaultPublicListPage     uint64 = 1                  // defaultPublicListPage 是公开列表默认页码。
	defaultPublicListPageSize uint64 = 10                 // defaultPublicListPageSize 是公开列表默认每页数量。
	publicSummaryRuneLimit           = 50                 // publicSummaryRuneLimit 是文章摘要最大 Unicode 字符数。
	hotRankQueryLimit                = 10                 // hotRankQueryLimit 是公开热榜最大返回数量。
	hotRankRebuildLimit              = 100                // hotRankRebuildLimit 是权威数据重建数量。
	viewEventProcessingTTL           = time.Minute        // viewEventProcessingTTL 是浏览事件处理中状态有效期。
	viewEventCompletedTTL            = 7 * 24 * time.Hour // viewEventCompletedTTL 是浏览事件完成标记有效期。
)

var (
	ErrInvalidViewEvent    = errors.New("浏览事件不合法")
	ErrViewEventProcessing = errors.New("浏览事件正在处理")
)

// PublicListCommand 表示公开文章列表分页输入。
type PublicListCommand struct {
	LastID   uint64 // LastID 是游标分页的上一页末尾文章标识，大于 0 时优先使用。
	Page     uint64 // Page 是 Offset 分页页码，0 使用默认值。
	PageSize uint64 // PageSize 是每页数量，0 使用默认值，非零范围为 10～20。
	IsDesc   bool   // IsDesc 表示是否按文章标识倒序查询。
}

// PublicListQuery 表示仓储使用的规范化公开列表查询。
type PublicListQuery struct {
	LastID   uint64 // LastID 是可选游标文章标识。
	Page     uint64 // Page 是规范化后的 Offset 页码。
	PageSize uint64 // PageSize 是规范化后的每页数量。
	IsDesc   bool   // IsDesc 表示是否按文章标识倒序查询。
}

// PublicListItem 表示公开文章列表的领域输出。
type PublicListItem struct {
	Article *entity.Article // Article 是已发表文章及其互动统计。
	Summary string          // Summary 是最多 50 个 Unicode 字符的正文摘要。
}

// PublicListResult 表示公开文章列表和分页元数据。
type PublicListResult struct {
	Items    []*PublicListItem // Items 是当前页公开文章。
	LastID   uint64            // LastID 是当前页最后一篇文章标识，无数据时为 0。
	Total    uint64            // Total 是已发表文章总数。
	Page     uint64            // Page 是规范化后的页码。
	PageSize uint64            // PageSize 是规范化后的每页数量。
}

// ViewEvent 表示文章详情触发的异步浏览事实。
type ViewEvent struct {
	EventID   string    `json:"event_id"`   // EventID 是跨重试保持稳定的幂等标识。
	ArticleID uint64    `json:"article_id"` // ArticleID 是被浏览文章标识。
	UserID    uint64    `json:"user_id"`    // UserID 是可选登录用户标识，0 表示游客。
	ViewedAt  time.Time `json:"viewed_at"`  // ViewedAt 是文章被访问的时间。
}

// HotMetric 表示文章热度权威字段。
type HotMetric struct {
	ArticleID    uint64 // ArticleID 是文章唯一标识。
	Title        string // Title 是文章标题。
	ViewCount    int64  // ViewCount 是浏览数投影。
	LikeCount    int64  // LikeCount 是点赞数投影。
	CommentCount int64  // CommentCount 是评论数投影。
}

// Score 返回当前文章热度：浏览数、点赞数和评论数之和。
func (m HotMetric) Score() int64 {
	return m.ViewCount + m.LikeCount + m.CommentCount
}

// RankEntry 表示 Redis 热榜中的文章标识和分值。
type RankEntry struct {
	ArticleID uint64  // ArticleID 是热榜文章标识。
	Score     float64 // Score 是 Redis Sorted Set 保存的热度分值。
}

// HotRankItem 表示公开热榜响应所需的数据。
type HotRankItem struct {
	Metric HotMetric // Metric 是文章标题和当前互动统计。
	Hot    int64     // Hot 是 Redis 排名使用的热度值。
}

// ViewEventState 表示浏览事件的幂等处理状态。
type ViewEventState int8

const (
	ViewEventNew        ViewEventState = 1 // ViewEventNew 表示当前消费者获得处理权。
	ViewEventProcessing ViewEventState = 2 // ViewEventProcessing 表示其他消费者正在处理。
	ViewEventCompleted  ViewEventState = 3 // ViewEventCompleted 表示事件已处理完成。
)

// ReadingRepository 定义公开阅读、浏览统计和热榜重建所需的数据能力。
type ReadingRepository interface {
	// ListPublished 分页查询已发表文章。
	ListPublished(context.Context, PublicListQuery) ([]*entity.Article, uint64, error)
	// RecordView 原子维护登录用户历史并增加文章浏览量。
	RecordView(context.Context, ViewEvent) (*HotMetric, error)
	// ViewEventProcessed 查询浏览事件是否已在 MySQL 事务中完成。
	ViewEventProcessed(context.Context, string) (bool, error)
	// FindHotMetric 查询指定已发表文章的权威热度字段。
	FindHotMetric(context.Context, uint64) (*HotMetric, error)
	// FindHotMetrics 批量查询指定已发表文章的权威热度字段。
	FindHotMetrics(context.Context, []uint64) ([]*HotMetric, error)
	// TopHotMetrics 查询热度最高的已发表文章。
	TopHotMetrics(context.Context, int) ([]*HotMetric, error)
}

// ViewEventPublisher 定义浏览事件异步发布能力。
type ViewEventPublisher interface {
	// PublishView 发布一次文章浏览事实。
	PublishView(context.Context, ViewEvent) error
}

// ViewDeadLetterPublisher 定义浏览事件处理失败后的死信投递能力。
type ViewDeadLetterPublisher interface {
	// PublishDeadLetter 将原始负载和失败原因投递到死信主题。
	PublishDeadLetter(context.Context, []byte, string) error
}

// ViewEventDeduplicator 定义浏览事件处理状态管理能力。
type ViewEventDeduplicator interface {
	// Begin 原子读取或占用浏览事件处理状态。
	Begin(context.Context, string, time.Duration) (ViewEventState, error)
	// Complete 将浏览事件标记为处理完成。
	Complete(context.Context, string, time.Duration) error
	// Release 在权威数据库写入失败时释放处理权。
	Release(context.Context, string) error
}

// HotRankStore 定义 Redis 热榜查询和投影维护能力。
type HotRankStore interface {
	// Top 查询热度最高的文章标识和分值。
	Top(context.Context, int64) ([]RankEntry, error)
	// SetScore 使用权威统计覆盖单篇文章热度。
	SetScore(context.Context, uint64, int64) error
	// Replace 使用 MySQL 权威统计原子覆盖热榜。
	Replace(context.Context, []*HotMetric) error
}

// ReadingUseCase 定义公开列表、浏览事件发布和热榜查询能力。
type ReadingUseCase interface {
	// ListPublished 查询已发表文章列表。
	ListPublished(context.Context, PublicListCommand) (*PublicListResult, error)
	// PublishView 发布文章详情浏览事件。
	PublishView(context.Context, uint64, uint64) error
	// HotRank 查询公开文章热榜。
	HotRank(context.Context) ([]*HotRankItem, error)
}

// ViewProcessor 定义浏览消费者处理能力。
type ViewProcessor interface {
	// ConsumeView 幂等维护浏览历史、浏览量和热榜投影。
	ConsumeView(context.Context, ViewEvent) error
}

// HotRankRebuilder 定义从 MySQL 权威数据重建 Redis 热榜的能力。
type HotRankRebuilder interface {
	// RebuildHotRank 重建最多 100 篇已发表文章的热榜投影。
	RebuildHotRank(context.Context) error
}

// ViewService 实现公开列表、浏览事件和热榜规则。
type ViewService struct {
	repository ReadingRepository     // repository 提供阅读场景的 MySQL 权威数据。
	publisher  ViewEventPublisher    // publisher 提供浏览事件异步发布能力。
	dedupe     ViewEventDeduplicator // dedupe 提供浏览事件幂等状态。
	hotRank    HotRankStore          // hotRank 提供 Redis 热榜投影。
	now        func() time.Time      // now 提供可测试的当前时间。
	newEventID func() string         // newEventID 提供可测试的浏览事件标识。
}

// NewViewService 创建公开阅读领域服务。
func NewViewService(repository ReadingRepository, publisher ViewEventPublisher, dedupe ViewEventDeduplicator, hotRank HotRankStore) *ViewService {
	// 1. 启动阶段拒绝缺少公开阅读所需依赖
	if repository == nil || publisher == nil || dedupe == nil || hotRank == nil {
		panic("文章阅读领域服务缺少必要依赖")
	}
	return &ViewService{repository: repository, publisher: publisher, dedupe: dedupe, hotRank: hotRank, now: time.Now, newEventID: uuid.NewString}
}

// ListPublished 查询已发表文章并生成兼容摘要。
func (s *ViewService) ListPublished(ctx context.Context, command PublicListCommand) (*PublicListResult, error) {
	// 1. 补齐并校验公开列表分页参数
	query, err := normalizePublicListQuery(command)
	if err != nil {
		return nil, err
	}

	// 2. 查询已发表文章并截取 Unicode 摘要
	articles, total, err := s.repository.ListPublished(ctx, query)
	if err != nil {
		return nil, err
	}
	items := make([]*PublicListItem, 0, len(articles))
	for _, article := range articles {
		items = append(items, &PublicListItem{Article: article, Summary: articleSummary(article.Content)})
	}
	lastID := uint64(0)
	if len(articles) > 0 {
		lastID = articles[len(articles)-1].ID
	}
	return &PublicListResult{Items: items, LastID: lastID, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

// PublishView 创建并发布文章浏览事件。
func (s *ViewService) PublishView(ctx context.Context, articleID, userID uint64) error {
	// 1. 为每次详情访问创建跨重试稳定的浏览事件
	event := ViewEvent{EventID: s.newEventID(), ArticleID: articleID, UserID: userID, ViewedAt: s.now()}
	return s.publisher.PublishView(ctx, event)
}

// ConsumeView 幂等维护浏览历史、浏览量和热榜投影。
func (s *ViewService) ConsumeView(ctx context.Context, event ViewEvent) error {
	// 1. 拒绝无法幂等或无法关联文章的浏览事件
	if event.EventID == "" || event.ArticleID == 0 || event.ViewedAt.IsZero() {
		return ErrInvalidViewEvent
	}

	// 2. 原子读取或占用事件处理状态
	state, err := s.dedupe.Begin(ctx, event.EventID, viewEventProcessingTTL)
	if err != nil {
		return fmt.Errorf("占用浏览事件处理状态: %w", err)
	}
	if state == ViewEventProcessing {
		processed, err := s.repository.ViewEventProcessed(ctx, event.EventID)
		if err != nil {
			return err
		}
		if processed {
			return s.repairHotRank(ctx, event.ArticleID, event.EventID)
		}
		return errors.Join(ErrViewEventProcessing, s.dedupe.Release(ctx, event.EventID))
	}
	if state == ViewEventCompleted {
		return s.repairHotRank(ctx, event.ArticleID, event.EventID)
	}

	// 3. 原子维护登录历史和文章浏览量，失败时释放处理权供 Kafka 重试
	metric, err := s.repository.RecordView(ctx, event)
	if err != nil {
		return errors.Join(err, s.dedupe.Release(ctx, event.EventID))
	}
	if err := s.dedupe.Complete(ctx, event.EventID, viewEventCompletedTTL); err != nil {
		return fmt.Errorf("完成浏览事件处理状态: %w", err)
	}

	// 4. 使用权威统计覆盖 Redis 热度，失败由重复事件路径修复
	if err := s.hotRank.SetScore(ctx, metric.ArticleID, metric.Score()); err != nil {
		return fmt.Errorf("更新文章热榜投影: %w", err)
	}
	return nil
}

// HotRank 查询 Redis 前十并按缓存顺序补充权威文章字段。
func (s *ViewService) HotRank(ctx context.Context) ([]*HotRankItem, error) {
	// 1. 从 Redis 读取前十文章标识和热度分值
	ranks, err := s.hotRank.Top(ctx, hotRankQueryLimit)
	if err != nil {
		return nil, err
	}
	articleIDs := make([]uint64, 0, len(ranks))
	for _, rank := range ranks {
		articleIDs = append(articleIDs, rank.ArticleID)
	}

	// 2. 批量查询已发表文章并按 Redis 排名顺序组装，跳过已不存在文章
	metrics, err := s.repository.FindHotMetrics(ctx, articleIDs)
	if err != nil {
		return nil, err
	}
	metricByID := make(map[uint64]*HotMetric, len(metrics))
	for _, metric := range metrics {
		metricByID[metric.ArticleID] = metric
	}
	items := make([]*HotRankItem, 0, len(ranks))
	for _, rank := range ranks {
		if metric := metricByID[rank.ArticleID]; metric != nil {
			items = append(items, &HotRankItem{Metric: *metric, Hot: int64(rank.Score)})
		}
	}
	return items, nil
}

// RebuildHotRank 使用 MySQL 权威统计覆盖 Redis 热榜前一百。
func (s *ViewService) RebuildHotRank(ctx context.Context) error {
	// 1. 查询权威热度最高的已发表文章
	metrics, err := s.repository.TopHotMetrics(ctx, hotRankRebuildLimit)
	if err != nil {
		return err
	}

	// 2. 原子替换 Redis 热榜投影
	return s.hotRank.Replace(ctx, metrics)
}

// repairHotRank 使用权威统计修复重复事件对应的 Redis 投影。
func (s *ViewService) repairHotRank(ctx context.Context, articleID uint64, eventID string) error {
	// 1. 重复事件不再增加浏览量，只覆盖可能失败的热榜投影
	metric, err := s.repository.FindHotMetric(ctx, articleID)
	if err != nil {
		return err
	}
	if err := s.hotRank.SetScore(ctx, articleID, metric.Score()); err != nil {
		return err
	}
	return s.dedupe.Complete(ctx, eventID, viewEventCompletedTTL)
}

// normalizePublicListQuery 补齐并校验公开列表分页参数。
func normalizePublicListQuery(command PublicListCommand) (PublicListQuery, error) {
	// 1. 缺省页码和每页数量使用兼容默认值
	if command.Page == 0 {
		command.Page = defaultPublicListPage
	}
	if command.PageSize == 0 {
		command.PageSize = defaultPublicListPageSize
	}
	if command.PageSize < 10 || command.PageSize > 20 {
		return PublicListQuery{}, ErrInvalidPagination
	}
	return PublicListQuery(command), nil
}

// articleSummary 按 Unicode 字符截取文章正文摘要。
func articleSummary(content string) string {
	// 1. 正文不超过限制时保持原文
	runes := []rune(content)
	if len(runes) <= publicSummaryRuneLimit {
		return content
	}

	// 2. 超出限制时截取前五十个字符并追加省略号
	return string(runes[:publicSummaryRuneLimit]) + "..."
}
