package service

import (
	"context"

	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/book"
	helloworldPb "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/helloworld"
	helloworld "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/leo/leo/log"
)

type AccountGrpcService struct {
	helloworldPb.UnimplementedGreeterServer
	hd *helloworld.HelloworldService
}

func NewGrpcBlogServer(hd *helloworld.HelloworldService) helloworldPb.GreeterServer {
	return &AccountGrpcService{
		hd: hd,
	}
}

func (a *AccountGrpcService) DoSome(ctx context.Context, req *helloworldPb.HelloRequest) (resp *helloworldPb.HelloReply, err error) {

	// 打印日志
	// log.WithCtx(ctx).Info("记录日志 ，随时打印traceid")

	log.Info("xxxx")

	// log.FromContext(ctx.Request.Context()).Info()

	// log.L().WithContext(ctx.Request.Context()).Info("记录日志 ，随时打印traceid")

	// 使用领域接口
	a.hd.Get("")
	return &helloworldPb.HelloReply{Message: "DoSome " + req.Name}, nil
}

func (a *AccountGrpcService) SayHello(ctx context.Context, req *helloworldPb.HelloRequest) (resp *helloworldPb.HelloReply, err error) {
	// 故意制造panic  测试sentry
	var ls []string
	ls[1] = "asd"
	return &helloworldPb.HelloReply{Message: "Hello " + req.Name}, nil
}

type BookGrpcService struct {
	book.UnimplementedBookServer
	// bk *helloworld.HelloworldService
	// book.UnimplementedBookServer
}

func (b *BookGrpcService) mustEmbedUnimplementedBookServer() {
	// TODO implement me
	panic("implement me")
}

func NewGrpcBookServer() book.BookServer {
	return &BookGrpcService{
		// hd: hd,
	}
}

func (b *BookGrpcService) ShowBook(ctx context.Context, req *book.RequestBook) (*book.ReplyBook, error) {
	log.Error("错误日志")
	log.FromContext(ctx).Info("xxx")
	log.L().WithContext(ctx).Info("ccc")
	return &book.ReplyBook{Message: "ok"}, nil
}

func (b *BookGrpcService) BookList(ctx context.Context, req *book.RequestBook) (*book.ReplyBook, error) {
	// log.L().WithContext(ctx.Request.Context()).Info(111)
	return &book.ReplyBook{
		Message: "33",
	}, nil
}

func (b *BookGrpcService) UpdateBook(ctx context.Context, req *book.RequestBook) (*book.ReplyBook, error) {
	log.L().WithContext(ctx).Info(111)
	return &book.ReplyBook{
		Message: "33",
	}, nil
}

func (b *BookGrpcService) DelBook(ctx context.Context, req *book.RequestBook) (*book.ReplyBook, error) {
	log.L().WithContext(ctx).Info(111)
	return nil, nil
}
