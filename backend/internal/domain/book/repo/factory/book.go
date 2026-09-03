package factory

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/entity"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book/repo/po"
	"codeup.aliyun.com/qimao/leo/leo/log"
	"github.com/jinzhu/copier"
)

func BookDto2Po(book *entity.Book) *po.Book {
	var bookPo po.Book
	err := copier.Copy(&bookPo, book)
	if err != nil {
		log.Error(err.Error())
	}
	return &bookPo
}

func BookPo2Dto(book *po.Book) *entity.Book {
	var bookDto entity.Book
	err := copier.Copy(&bookDto, book)
	if err != nil {
		log.Error(err.Error())
	}
	return &bookDto
}
