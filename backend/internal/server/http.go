package server

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/helloworld"
	"codeup.aliyun.com/qimao/leo/leo/transport/ginhttp"
	gonicgin "github.com/gin-gonic/gin"
)

func NewHTTPServer(helloworldServer helloworld.GreeterHTTPServerController, bookServer book.BookHTTPServerController) ginhttp.RegisterServer {
	return &httpServer{
		helloworldServer: helloworldServer,
		bookServer:       bookServer,
	}
}

type httpServer struct {
	helloworldServer helloworld.GreeterHTTPServerController
	bookServer       book.BookHTTPServerController
}

func (srv *httpServer) Register(engine *gonicgin.Engine) {
	routerGroup := engine.Group("")
	// book.
	helloworld.RegisterGreeterHTTPServerController(routerGroup, srv.helloworldServer)
	book.RegisterBookHTTPServerController(routerGroup, srv.bookServer)

}
