package repo

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/clients"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/repo/factory"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/repo/po"
	"codeup.aliyun.com/qimao/leo/leo/log"

	"github.com/samber/lo"
)

type HdRepo struct {
	client    clients.MysqlClient
	logClient clients.MysqlLogClient
	rdClient  clients.RedisClient
}

func NewHdRepo(client clients.MysqlClient, logClient clients.MysqlLogClient, rdClient clients.RedisClient) book.HdRepo {
	return &HdRepo{
		client:    client,
		logClient: logClient,
		rdClient:  rdClient,
	}
}

func (h *HdRepo) SaveOne(ctx context.Context, data *entity.Book) (int64, error) {
	dataPo := factory.BookDto2Po(data)
	row, err := h.client.Context(ctx).Insert(dataPo)
	if err != nil {
		log.L().WithContext(ctx).Error(err.Error())
		return 0, err
	}
	return row, nil
}

func (h *HdRepo) GetList(ctx context.Context, id string) ([]*entity.Book, error) {
	list := []*po.Book{}
	err := h.client.Context(ctx).Select("id,title").Where("id>=?", id).Limit(20).Find(&list)
	if err != nil {
		log.L().WithContext(ctx).Error(err)
		return nil, err
	}
	listDto := lo.Map[*po.Book, *entity.Book](list, func(do *po.Book, _ int) *entity.Book {
		return factory.BookPo2Dto(do)
	})
	return listDto, nil
}

func (h *HdRepo) EditOne(ctx context.Context, data *entity.Book) (*entity.Book, error) {
	dataPo := factory.BookDto2Po(data)

	row, err := h.client.Context(ctx).UseBool("title", "price").Update(dataPo)
	if err != nil {
		log.L().WithContext(ctx).Error(err)
		return nil, err
	}
	if row == 0 {
		log.L().WithContext(ctx).Error("更新失败")
	}
	return data, nil
}

func (h *HdRepo) DeleteId(ctx context.Context, id int64) (int64, error) {
	dataPo := &po.Book{}
	row, err := h.client.Context(ctx).Where("id=?", id).Delete(dataPo)
	if err != nil {
		log.L().WithContext(ctx).Error(err)
		return row, err
	}
	return row, nil
}
