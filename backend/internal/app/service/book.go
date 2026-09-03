package service

import (
	"codeup.aliyun.com/qimao/leo/leo/log"
	"github.com/gin-gonic/gin"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/book"
)

type BookService struct {
	// bk *helloworld.HelloworldService
	// book.UnimplementedBookServer
}

func NewBookServer() book.BookHTTPServerController {
	return &BookService{
		// hd: hd,
	}
}

func (BookService) ShowBook(ctx *gin.Context, req *book.RequestBook) (*book.ReplyBook, error) {
	log.Error("错误日志")
	log.FromContext(ctx).Info("xxx")
	log.L().WithContext(ctx).Info("ccc")
	return &book.ReplyBook{Message: "ok"}, nil
}

func (BookService) BookList(ctx *gin.Context, req *book.RequestBook) (*book.ReplyBook, error) {
	log.L().WithContext(ctx.Request.Context()).Info(111)
	return &book.ReplyBook{
		Message: "33",
	}, nil
}

func (BookService) UpdateBook(ctx *gin.Context, req *book.RequestBook) (*book.ReplyBook, error) {
	log.L().WithContext(ctx).Info(111)
	return &book.ReplyBook{
		Message: "33",
	}, nil
}

func (BookService) DelBook(ctx *gin.Context, req *book.RequestBook) (*book.ReplyBook, error) {
	log.L().WithContext(ctx).Info(111)
	return nil, nil
}
