package book

import (
	"context"
	"log"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/entity"
)

// Service server服务
type HelloworldService struct {
	hdRepo HdRepo
}

type HdRepo interface {
	SaveOne(ctx context.Context, data *entity.Book) (int64, error)
	GetList(ctx context.Context, id string) ([]*entity.Book, error)
	EditOne(ctx context.Context, data *entity.Book) (*entity.Book, error)
	DeleteId(ctx context.Context, id int64) (int64, error)
}

// NewHelloworld wire注入用
func NewHelloworld(hdRepo HdRepo) *HelloworldService {
	return &HelloworldService{
		hdRepo: hdRepo,
	}
}

func (h HelloworldService) Get(key string) string {
	// TODO implement me
	log.Println("implement me")
	return ""
}

func (h HelloworldService) Set(key string, val string) {
	// TODO implement me
	log.Println("implement me")
}

func (h HelloworldService) Job() error {
	// TODO implement me
	log.Println("implement me")
	return nil
}
