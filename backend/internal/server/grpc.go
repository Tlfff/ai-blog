package server

import (
	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/helloworld"
	"codeup.aliyun.com/qimao/leo/leo/transport/lgrpc"
)

// NewGrpcServer 创建只负责注册生成接口的 gRPC 服务聚合器。
func NewGrpcServer(helloworldServer helloworld.GreeterServer, bookServer book.BookServer, userServer blogopenv1.UserServiceServer) *GrpcServer {
	// 1. 保存各生成服务实现，注册动作延迟到 Leo Server 初始化
	return &GrpcServer{
		helloworldServer: helloworldServer,
		bookServer:       bookServer,
		userServer:       userServer,
	}
}

// GrpcServer 聚合当前进程需要注册的生成 gRPC 服务。
type GrpcServer struct {
	helloworldServer helloworld.GreeterServer     // helloworldServer 是存量示例服务。
	bookServer       book.BookServer              // bookServer 是存量图书示例服务。
	userServer       blogopenv1.UserServiceServer // userServer 是开放用户查询服务。
}

// Register 将生成服务注册到 Leo gRPC Server。
func (srv *GrpcServer) Register(gr *lgrpc.Server) {
	// 1. 保留存量服务并追加开放用户查询服务
	helloworld.RegisterGreeterServer(gr, srv.helloworldServer)
	book.RegisterBookServer(gr, srv.bookServer)
	blogopenv1.RegisterUserServiceServer(gr, srv.userServer)
}
