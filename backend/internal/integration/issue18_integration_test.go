//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/consumer"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/app/job"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients/eventstream"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	articlerepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment"
	commententity "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/entity"
	commentrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/comment/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like"
	likerepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/like/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification"
	notificationrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/notification/repo"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	userrepo "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user/repo"
	"codeup.aliyun.com/qimao/leo/leo/stream"
	confluent "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"xorm.io/xorm"
)

const (
	commentTopic = "issue18-comment-events"
	commentDLQ   = "issue18-comment-events-dlq"
	likeTopic    = "issue18-like-events"
	likeDLQ      = "issue18-like-events-dlq"
	viewTopic    = "issue18-view-events"
	viewDLQ      = "issue18-view-events-dlq"
)

func TestIssue18CrossContextEventChainsAndRecovery(t *testing.T) {
	if os.Getenv("ISSUE18_INTEGRATION") != "1" {
		t.Skip("set ISSUE18_INTEGRATION=1 to run real infrastructure acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	engine := openMySQL(t)
	defer engine.Close()
	redisClient := openRedis(t)
	defer redisClient.Close()
	mongoClient := openMongo(t)
	defer mongoClient.Disconnect(context.Background())
	seedBusinessFacts(t, engine)
	if err := redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	if err := mongoClient.Database("issue18").Collection("notifications").Drop(ctx); err != nil && !errors.Is(err, mongo.ErrNilDocument) {
		t.Fatal(err)
	}

	articleRepository := articlerepo.NewRepository(engine, articlerepo.ProvideTransactionClient(engine))
	commentRepository := commentrepo.NewRepository(engine, commentrepo.ProvideTransactionClient(engine))
	likeRepository := likerepo.NewRepository(engine, likerepo.ProvideTransactionClient(engine))
	userRepository := userrepo.NewUserRepository(engine, userrepo.ProvideTransactionClient(engine))
	userService := user.NewService(userRepository, user.NewPBKDF2PasswordHasher())
	readingCache := articlerepo.NewReadingCache(redisClient)

	mongoRepository, err := notificationrepo.NewRepository(&clients.MongoClient{Client: mongoClient, Database: "issue18"})
	if err != nil {
		t.Fatal(err)
	}
	articleNotificationReader := notificationrepo.NewArticleReader(article.NewNotificationQuery(articleRepository))
	userNotificationReader := notificationrepo.NewUserReader(userService)
	notificationService := notification.NewService(mongoRepository, articleNotificationReader, userNotificationReader)

	kafkaConfig := newKafkaConfig(requiredEnv(t, "ISSUE18_KAFKA_BOOTSTRAP"))
	commentPublisher, err := eventstream.NewCommentEventPublisher(kafkaConfig)
	if err != nil {
		t.Fatal(err)
	}
	commentDeadLetter, err := eventstream.NewCommentEventDeadLetterPublisher(kafkaConfig)
	if err != nil {
		t.Fatal(err)
	}
	likePublisher, err := eventstream.NewLikeEventPublisher(kafkaConfig)
	if err != nil {
		t.Fatal(err)
	}
	likeDeadLetter, err := eventstream.NewLikeEventDeadLetterPublisher(kafkaConfig)
	if err != nil {
		t.Fatal(err)
	}
	viewPublisher, err := eventstream.NewArticleViewPublisher(kafkaConfig)
	if err != nil {
		t.Fatal(err)
	}
	viewDeadLetter, err := eventstream.NewArticleViewDeadLetterPublisher(kafkaConfig)
	if err != nil {
		t.Fatal(err)
	}
	startRunners(t, commentPublisher, commentDeadLetter, likePublisher, likeDeadLetter, viewPublisher, viewDeadLetter)

	var commentID uint64
	t.Run("comment outbox retries then updates article count idempotently", func(t *testing.T) {
		created := &commententity.Comment{ArticleID: 1, UserID: 2, Content: "真实评论链路", Status: comment.StatusNormal, CreatedTime: time.Now(), UpdatedTime: time.Now()}
		if err := commentRepository.Create(ctx, created); err != nil {
			t.Fatal(err)
		}
		commentID = created.ID
		flakyPublisher := &failOnceCommentPublisher{next: commentPublisher}
		relay := job.NewCommentOutboxRelay(commentRepository, flakyPublisher)
		if err := relay.PublishBatch(ctx); err == nil {
			t.Fatal("temporary Kafka failure was not persisted")
		}
		assertOutboxRetry(t, engine, "comment_event_outbox")
		if _, err := engine.Exec("UPDATE comment_event_outbox SET next_attempt_time = CURRENT_TIMESTAMP WHERE status = 0"); err != nil {
			t.Fatal(err)
		}
		if err := relay.PublishBatch(ctx); err != nil {
			t.Fatal(err)
		}
		payload := consumeKafka(t, commentTopic, "comment-count")
		projector := article.NewCommentCountProjector(articleRepository)
		flakyProcessor := &failOnceCommentProcessor{next: projector}
		handler := consumer.NewCommentCountConsumer(noopSubscriber{}, flakyProcessor, commentDeadLetter)
		message := &stream.Message{Payload: payload}
		if err := handler.Handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		if flakyProcessor.calls != 2 {
			t.Fatalf("consumer calls=%d, want retry success", flakyProcessor.calls)
		}
		if err := handler.Handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		assertArticleCounts(t, engine, 1, 1, 0, 0)
	})

	t.Run("article like updates count and one Mongo notification", func(t *testing.T) {
		changed, err := likeRepository.ChangeArticleLike(ctx, 2, 1, like.StatusLiked, time.Now())
		if err != nil || !changed {
			t.Fatalf("changed=%v error=%v", changed, err)
		}
		if err := job.NewLikeOutboxRelay(likeRepository, likePublisher).PublishBatch(ctx); err != nil {
			t.Fatal(err)
		}
		countPayload := consumeKafka(t, likeTopic, "like-count")
		notificationPayload := consumeKafka(t, likeTopic, "like-notification")
		likeHandler := consumer.NewLikeCountConsumer(noopSubscriber{}, article.NewLikeCountProjector(articleRepository), comment.NewLikeCountProjector(commentRepository), likeDeadLetter)
		if err := likeHandler.Handle(ctx, &stream.Message{Payload: countPayload}); err != nil {
			t.Fatal(err)
		}
		if err := likeHandler.Handle(ctx, &stream.Message{Payload: countPayload}); err != nil {
			t.Fatal(err)
		}
		flakyNotification := &failOnceNotificationProcessor{next: notificationService}
		notificationHandler := consumer.NewNotificationConsumer(noopSubscriber{}, flakyNotification, likeDeadLetter)
		if err := notificationHandler.Handle(ctx, &stream.Message{Payload: notificationPayload}); err != nil {
			t.Fatal(err)
		}
		if flakyNotification.calls != 2 {
			t.Fatalf("notification calls=%d, want retry success", flakyNotification.calls)
		}
		if err := notificationHandler.Handle(ctx, &stream.Message{Payload: notificationPayload}); err != nil {
			t.Fatal(err)
		}
		assertArticleCounts(t, engine, 1, 1, 1, 0)
		result, err := notificationService.List(ctx, notification.PageQuery{UserID: 1, Page: 1, PageSize: 10})
		if err != nil || len(result.Items) != 1 {
			t.Fatalf("notifications=%#v error=%v", result, err)
		}
		item := result.Items[0]
		if item.SenderNickname != "读者" || item.Title != "集成验收文章" || item.ReceiverID != 1 {
			t.Fatalf("notification=%#v", item)
		}
	})

	t.Run("comment like updates count and projection can be rebuilt", func(t *testing.T) {
		changed, err := likeRepository.ChangeCommentLike(ctx, 1, commentID, like.StatusLiked, time.Now())
		if err != nil || !changed {
			t.Fatalf("changed=%v error=%v", changed, err)
		}
		if err := job.NewCommentLikeOutboxRelay(likeRepository, likePublisher).PublishBatch(ctx); err != nil {
			t.Fatal(err)
		}
		payload := consumeKafkaMatching(t, likeTopic, "comment-like-count", func(message *confluent.Message) bool {
			var event like.IntegrationEvent
			return json.Unmarshal(message.Value, &event) == nil && event.EventType == like.CommentLikedEventType
		})
		projector := comment.NewLikeCountProjector(commentRepository)
		handler := consumer.NewLikeCountConsumer(noopSubscriber{}, article.NewLikeCountProjector(articleRepository), projector, likeDeadLetter)
		message := &stream.Message{Payload: payload}
		if err := handler.Handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		if err := handler.Handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		assertCommentLikeCount(t, engine, commentID, 1)
		if _, err := engine.Exec("UPDATE comments SET like_count = 77 WHERE id = ?", commentID); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Exec("DELETE FROM comment_like_projection"); err != nil {
			t.Fatal(err)
		}
		if err := projector.RebuildCommentLikeCount(ctx); err != nil {
			t.Fatal(err)
		}
		assertCommentLikeCount(t, engine, commentID, 1)
	})
	t.Run("dead letter is published after retries are exhausted", func(t *testing.T) {
		event := article.CommentCountEvent{EventID: "dead-letter-event", EventType: article.CommentCreatedEventType, Version: 1, OccurredAt: time.Now(), AggregateID: 99, CommentID: 99, ArticleID: 1}
		payload, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		handler := consumer.NewCommentCountConsumer(noopSubscriber{}, alwaysFailCommentProcessor{}, commentDeadLetter)
		if err := handler.Handle(ctx, &stream.Message{Payload: payload}); err != nil {
			t.Fatal(err)
		}
		dead := consumeKafkaMessage(t, commentDLQ, "comment-dlq")
		if string(dead.Value) != string(payload) || kafkaHeader(dead, "x-dead-letter-error") == "" {
			t.Fatalf("dead letter payload=%q headers=%#v", dead.Value, dead.Headers)
		}
	})

	t.Run("reconciliation repairs corrupted article interaction projections", func(t *testing.T) {
		for _, statement := range []string{
			"UPDATE articles SET comment_count = 41, like_count = 42 WHERE id = 1",
			"DELETE FROM article_comment_projection",
			"DELETE FROM article_like_projection",
		} {
			if _, err := engine.Exec(statement); err != nil {
				t.Fatal(err)
			}
		}
		service := article.NewInteractionProjectionService(articleRepository)
		reconciler := job.NewArticleInteractionReconcileJob(commentRepository, likeRepository, service)
		if err := reconciler.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		assertArticleCounts(t, engine, 1, 1, 1, 0)
	})

	t.Run("view event creates history and hot rank is rebuilt", func(t *testing.T) {
		event := article.ViewEvent{EventID: "view-event-1", ArticleID: 1, UserID: 2, ViewedAt: time.Now()}
		if err := viewPublisher.PublishView(ctx, event); err != nil {
			t.Fatal(err)
		}
		payload := consumeKafka(t, viewTopic, "view-count")
		viewService := article.NewViewService(articleRepository, viewPublisher, readingCache, readingCache)
		flakyProcessor := &failOnceViewProcessor{next: viewService}
		handler := consumer.NewArticleViewConsumer(noopSubscriber{}, flakyProcessor, viewDeadLetter)
		message := &stream.Message{Payload: payload}
		if err := handler.Handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		if flakyProcessor.calls != 2 {
			t.Fatalf("view calls=%d, want retry success", flakyProcessor.calls)
		}
		if err := handler.Handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		assertArticleCounts(t, engine, 1, 1, 1, 1)
		var histories int64
		if _, err := engine.SQL("SELECT COUNT(*) FROM article_view_histories WHERE user_id = 2 AND article_id = 1").Get(&histories); err != nil || histories != 1 {
			t.Fatalf("histories=%d error=%v", histories, err)
		}
		if err := redisClient.ZAdd(ctx, "article:hot-rank", redis.Z{Score: 999, Member: "1"}).Err(); err != nil {
			t.Fatal(err)
		}
		if err := viewService.RebuildHotRank(ctx); err != nil {
			t.Fatal(err)
		}
		score, err := redisClient.ZScore(ctx, "article:hot-rank", "1").Result()
		if err != nil || score != 3 {
			t.Fatalf("hot score=%v error=%v", score, err)
		}
	})
}

func openMySQL(t *testing.T) *xorm.Engine {
	t.Helper()
	engine, err := xorm.NewEngine("mysql", requiredEnv(t, "ISSUE18_MYSQL_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Ping(); err != nil {
		engine.Close()
		t.Fatal(err)
	}
	return engine
}

func openRedis(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: requiredEnv(t, "ISSUE18_REDIS_ADDR")})
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		t.Fatal(err)
	}
	return client
}

func openMongo(t *testing.T) *mongo.Client {
	t.Helper()
	client, err := mongo.Connect(options.Client().ApplyURI(requiredEnv(t, "ISSUE18_MONGO_URI")))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		client.Disconnect(context.Background())
		t.Fatal(err)
	}
	return client
}

func seedBusinessFacts(t *testing.T, engine *xorm.Engine) {
	t.Helper()
	for _, statement := range []string{
		"DELETE FROM comment_like_projection", "DELETE FROM comment_like_event_inbox", "DELETE FROM comment_like_event_outbox", "DELETE FROM comment_likes",
		"DELETE FROM article_like_projection", "DELETE FROM article_like_event_inbox", "DELETE FROM article_like_event_outbox", "DELETE FROM article_likes",
		"DELETE FROM article_comment_projection", "DELETE FROM article_comment_event_inbox", "DELETE FROM comment_event_outbox", "DELETE FROM comments",
		"DELETE FROM article_view_event_inbox", "DELETE FROM article_view_histories", "DELETE FROM articles", "DELETE FROM users",
	} {
		if _, err := engine.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.Exec(`INSERT INTO users (id,nickname,phone,password,avatar,role,status,last_login_ip,last_login_time) VALUES
(1,'作者','13800000001','hash','/author.png',2,1,'',CURRENT_TIMESTAMP),
(2,'读者','13800000002','hash','/reader.png',1,1,'',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec(`INSERT INTO articles (id,author_id,title,content,tags,status,view_count,like_count,comment_count) VALUES (1,1,'集成验收文章','正文','Go',3,0,0,0)`); err != nil {
		t.Fatal(err)
	}
}

func newKafkaConfig(bootstrap string) *conf.Config {
	producer := func(topic string) *conf.KafkaProducer_Config {
		return &conf.KafkaProducer_Config{BootstrapServers: bootstrap, Topic: topic}
	}
	return &conf.Config{Data: &conf.Data{Kafka: &conf.Kafka{Producer: &conf.KafkaProducer{
		CommentEvent: producer(commentTopic), CommentEventDeadLetter: producer(commentDLQ),
		LikeEvent: producer(likeTopic), LikeEventDeadLetter: producer(likeDLQ),
		ArticleView: producer(viewTopic), ArticleViewDeadLetter: producer(viewDLQ),
	}}}}
}

func startRunners(t *testing.T, runners ...interface{ Run(context.Context) error }) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, len(runners))
	for _, runner := range runners {
		go func(current interface{ Run(context.Context) error }) { done <- current.Run(ctx) }(runner)
	}
	t.Cleanup(func() {
		cancel()
		for range runners {
			select {
			case err := <-done:
				if err != nil && !errors.Is(err, context.Canceled) {
					t.Errorf("runner shutdown: %v", err)
				}
			case <-time.After(15 * time.Second):
				t.Error("runner shutdown timed out")
			}
		}
	})
}

func consumeKafka(t *testing.T, topic, group string) []byte {
	t.Helper()
	return append([]byte(nil), consumeKafkaMessage(t, topic, group).Value...)
}

func consumeKafkaMatching(t *testing.T, topic, group string, matches func(*confluent.Message) bool) []byte {
	t.Helper()
	return append([]byte(nil), consumeKafkaMessageMatching(t, topic, group, matches).Value...)
}

func consumeKafkaMessage(t *testing.T, topic, group string) *confluent.Message {
	t.Helper()
	return consumeKafkaMessageMatching(t, topic, group, func(*confluent.Message) bool { return true })
}

func consumeKafkaMessageMatching(t *testing.T, topic, group string, matches func(*confluent.Message) bool) *confluent.Message {
	t.Helper()
	consumer, err := confluent.NewConsumer(&confluent.ConfigMap{
		"bootstrap.servers": requiredEnv(t, "ISSUE18_KAFKA_BOOTSTRAP"),
		"group.id":          fmt.Sprintf("issue18-%s-%d", group, time.Now().UnixNano()),
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	if err := consumer.SubscribeTopics([]string{topic}, nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		event := consumer.Poll(500)
		switch current := event.(type) {
		case *confluent.Message:
			if matches(current) {
				return current
			}
		case confluent.Error:
			if current.IsFatal() {
				t.Fatal(current)
			}
		}
	}
	t.Fatalf("timed out consuming topic %s", topic)
	return nil
}

func kafkaHeader(message *confluent.Message, key string) string {
	for _, header := range message.Headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}

func assertOutboxRetry(t *testing.T, engine *xorm.Engine, table string) {
	t.Helper()
	var row struct {
		Status   int8 `xorm:"'status'"`
		Attempts int  `xorm:"'attempts'"`
	}
	found, err := engine.SQL("SELECT status, attempts FROM " + table + " LIMIT 1").Get(&row)
	if err != nil || !found || row.Status != 0 || row.Attempts != 1 {
		t.Fatalf("outbox=%#v found=%v error=%v", row, found, err)
	}
}

func assertArticleCounts(t *testing.T, engine *xorm.Engine, articleID uint64, comments, likes, views int64) {
	t.Helper()
	var row struct {
		CommentCount int64 `xorm:"'comment_count'"`
		LikeCount    int64 `xorm:"'like_count'"`
		ViewCount    int64 `xorm:"'view_count'"`
	}
	found, err := engine.SQL("SELECT comment_count, like_count, view_count FROM articles WHERE id = ?", articleID).Get(&row)
	if err != nil || !found || row.CommentCount != comments || row.LikeCount != likes || row.ViewCount != views {
		t.Fatalf("counts=%#v found=%v error=%v", row, found, err)
	}
}

func assertCommentLikeCount(t *testing.T, engine *xorm.Engine, commentID uint64, want int64) {
	t.Helper()
	var count int64
	found, err := engine.SQL("SELECT like_count FROM comments WHERE id = ?", commentID).Get(&count)
	if err != nil || !found || count != want {
		t.Fatalf("comment like_count=%d found=%v error=%v", count, found, err)
	}
}

func requiredEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}

type noopSubscriber struct{}

func (noopSubscriber) Topic() string { return "noop" }
func (noopSubscriber) Queue() string { return "noop" }
func (noopSubscriber) Subscribe(context.Context, chan<- *stream.Message, chan<- error) error {
	return nil
}
func (noopSubscriber) Close(context.Context) error { return nil }

type failOnceCommentPublisher struct {
	next  comment.EventPublisher
	calls int
}

func (f *failOnceCommentPublisher) Publish(ctx context.Context, event comment.IntegrationEvent) error {
	f.calls++
	if f.calls == 1 {
		return errors.New("temporary kafka failure")
	}
	return f.next.Publish(ctx, event)
}

type failOnceCommentProcessor struct {
	next  article.CommentCountProcessor
	calls int
}

func (f *failOnceCommentProcessor) ApplyCommentCountEvent(ctx context.Context, event article.CommentCountEvent) error {
	f.calls++
	if f.calls == 1 {
		return errors.New("temporary mysql failure")
	}
	return f.next.ApplyCommentCountEvent(ctx, event)
}

type alwaysFailCommentProcessor struct{}

func (alwaysFailCommentProcessor) ApplyCommentCountEvent(context.Context, article.CommentCountEvent) error {
	return errors.New("persistent projection failure")
}

type failOnceNotificationProcessor struct {
	next  notification.Processor
	calls int
}

func (f *failOnceNotificationProcessor) ConsumeArticleLike(ctx context.Context, event like.IntegrationEvent) error {
	f.calls++
	if f.calls == 1 {
		return errors.New("temporary mongo failure")
	}
	return f.next.ConsumeArticleLike(ctx, event)
}

type failOnceViewProcessor struct {
	next  article.ViewProcessor
	calls int
}

func (f *failOnceViewProcessor) ConsumeView(ctx context.Context, event article.ViewEvent) error {
	f.calls++
	if f.calls == 1 {
		return errors.New("temporary redis failure")
	}
	return f.next.ConsumeView(ctx, event)
}
