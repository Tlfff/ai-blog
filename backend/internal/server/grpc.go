package server

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/helloworld"
	"codeup.aliyun.com/qimao/leo/leo/transport/lgrpc"
)

func NewGrpcServer(helloworldServer helloworld.GreeterServer, bookServer book.BookServer) *GrpcServer {
	return &GrpcServer{
		helloworldServer: helloworldServer,
		bookServer:       bookServer,
	}
}

type GrpcServer struct {
	helloworldServer helloworld.GreeterServer
	bookServer       book.BookServer
}

func (srv *GrpcServer) Register(gr *lgrpc.Server) {
	helloworld.RegisterGreeterServer(gr, srv.helloworldServer)
	book.RegisterBookServer(gr, srv.bookServer)
}
