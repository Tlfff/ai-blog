package service

import (
	helloworldPb "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/helloworld"
	helloworld "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/book"
	"codeup.aliyun.com/qimao/leo/leo/log"
	gonicgin "github.com/gin-gonic/gin"
)

type AccountService struct {
	hd *helloworld.HelloworldService
}

func NewBlogServer(hd *helloworld.HelloworldService) helloworldPb.GreeterHTTPServerController {
	return &AccountService{
		hd: hd,
	}
}

func (a *AccountService) DoSome(ctx *gonicgin.Context, req *helloworldPb.HelloRequest) (resp *helloworldPb.HelloReply, err error) {

	// 打印日志
	// log.WithCtx(ctx).Info("记录日志 ，随时打印traceid")

	log.Info("xxxx")

	log.FromContext(ctx.Request.Context()).Info()

	// log.L().WithContext(ctx.Request.Context()).Info("记录日志 ，随时打印traceid")

	// 使用领域接口
	a.hd.Get("")
	return &helloworldPb.HelloReply{Message: "DoSome " + req.Name}, nil
}

func (a *AccountService) SayHello(ctx *gonicgin.Context, req *helloworldPb.HelloRequest) (resp *helloworldPb.HelloReply, err error) {
	// 故意制造panic  测试sentry
	var ls []string
	ls[1] = "asd"
	return &helloworldPb.HelloReply{Message: "Hello " + req.Name}, nil
}
