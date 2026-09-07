package job

import (
	"context"
	"fmt"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

const articleInteractionReconcileInterval = 10 * time.Minute

// ArticleInteractionReconcileJob 从评论和点赞权威事实修复文章互动投影。
type ArticleInteractionReconcileJob struct {
	comments comment.ArticleProjectionFactReader   // comments 提供评论上下文稳定快照。
	likes    like.ArticleProjectionFactReader      // likes 提供点赞上下文稳定快照。
	repairer article.InteractionProjectionRepairer // repairer 只写文章上下文拥有的投影。
	interval time.Duration                         // interval 是两次对账的固定间隔。
	now      func() time.Time                      // now 提供可测试的快照起始时间。
}

// NewArticleInteractionReconcileJob 创建跨上下文只读对账任务。
func NewArticleInteractionReconcileJob(comments comment.ArticleProjectionFactReader, likes like.ArticleProjectionFactReader, repairer article.InteractionProjectionRepairer) *ArticleInteractionReconcileJob {
	if comments == nil || likes == nil || repairer == nil {
		panic("文章互动投影对账任务缺少必要依赖")
	}
	return &ArticleInteractionReconcileJob{comments: comments, likes: likes, repairer: repairer, interval: articleInteractionReconcileInterval, now: time.Now}
}

// Reconcile 读取两个事实源的完整快照并交给文章上下文原子修复。
func (j *ArticleInteractionReconcileJob) Reconcile(ctx context.Context) error {
	// 1. 在读取任一事实源前记录快照边界，供文章仓储保护并发新事件
	capturedAt := j.now()
	// 2. 仅通过评论和点赞上下文公开的稳定只读契约收集权威事实
	commentFacts, err := j.comments.ListArticleProjectionFacts(ctx)
	if err != nil {
		return fmt.Errorf("读取评论计数对账快照: %w", err)
	}
	likeFacts, err := j.likes.ListArticleProjectionFacts(ctx)
	if err != nil {
		return fmt.Errorf("读取文章点赞对账快照: %w", err)
	}
	// 3. 转换为文章上下文自己的快照类型，避免共享领域模型
	snapshot := article.InteractionProjectionSnapshot{
		CapturedAt: capturedAt,
		Comments:   make([]article.CommentProjectionFact, 0, len(commentFacts)),
		Likes:      make([]article.LikeProjectionFact, 0, len(likeFacts)),
	}
	for _, fact := range commentFacts {
		snapshot.Comments = append(snapshot.Comments, article.CommentProjectionFact{
			CommentID: fact.CommentID,
			ArticleID: fact.ArticleID,
			Version:   fact.Version,
			Active:    fact.Active,
		})
	}
	for _, fact := range likeFacts {
		snapshot.Likes = append(snapshot.Likes, article.LikeProjectionFact{
			LikeID:    fact.LikeID,
			ArticleID: fact.ArticleID,
			UserID:    fact.UserID,
			Version:   fact.Version,
			Active:    fact.Active,
		})
	}
	// 4. 文章上下文在单个本地事务中合并关系状态并重算计数
	if err := j.repairer.RepairInteractionProjections(ctx, snapshot); err != nil {
		return fmt.Errorf("修复文章互动投影: %w", err)
	}
	return nil
}

// Run 启动时和固定周期执行对账，单轮失败保留到下一轮继续修复。
func (j *ArticleInteractionReconcileJob) Run(ctx context.Context) error {
	// 1. 启动时立即修复一次，失败不阻断同步业务入口
	j.reconcileAndLog(ctx)
	// 2. 后续周期执行，退出由 Leo 统一生命周期控制
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			j.reconcileAndLog(ctx)
		}
	}
}

// reconcileAndLog 记录单轮失败并保留到下个周期继续修复。
func (j *ArticleInteractionReconcileJob) reconcileAndLog(ctx context.Context) {
	// 1. 对账错误不终止 HTTP 进程，避免修复链路反向影响同步接口
	if err := j.Reconcile(ctx); err != nil && ctx.Err() == nil {
		log.L().WithContext(ctx).Error("对账文章互动投影失败", err)
	}
}
