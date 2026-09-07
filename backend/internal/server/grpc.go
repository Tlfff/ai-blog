package server

import (
	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/helloworld"
	"codeup.aliyun.com/qimao/leo/leo/transport/lgrpc"
)

// NewGrpcServer 创建只负责注册生成接口的 gRPC 服务聚合器。
//
// 参数说明：
//   - helloworldServer：存量示例 gRPC 服务。
//   - bookServer：存量图书 gRPC 服务。
//   - userServer：开放用户查询服务。
//   - articleServer：开放文章列表查询服务。
//   - commentServer：开放评论统计查询服务。
func NewGrpcServer(
	helloworldServer helloworld.GreeterServer,
	bookServer book.BookServer,
	userServer blogopenv1.UserServiceServer,
	articleServer blogopenv1.ArticleServiceServer,
	commentServer blogopenv1.CommentServiceServer,
) *GrpcServer {
	// 1. 保存各生成服务实现，注册动作延迟到 Leo Server 初始化
	if helloworldServer == nil || bookServer == nil || userServer == nil || articleServer == nil || commentServer == nil {
		panic("gRPC 服务聚合器缺少必要依赖")
	}
	return &GrpcServer{helloworldServer: helloworldServer, bookServer: bookServer, userServer: userServer, articleServer: articleServer, commentServer: commentServer}
}

// GrpcServer 聚合当前进程需要注册的生成 gRPC 服务。
type GrpcServer struct {
	helloworldServer helloworld.GreeterServer        // helloworldServer 是存量示例服务。
	bookServer       book.BookServer                 // bookServer 是存量图书示例服务。
	userServer       blogopenv1.UserServiceServer    // userServer 是开放用户查询服务。
	articleServer    blogopenv1.ArticleServiceServer // articleServer 是开放文章查询服务。
	commentServer    blogopenv1.CommentServiceServer // commentServer 是开放评论查询服务。
}

// Register 将生成服务注册到 Leo gRPC Server。
func (srv *GrpcServer) Register(gr *lgrpc.Server) {
	// 1. 保留存量服务并注册全部开放上下文查询服务
	helloworld.RegisterGreeterServer(gr, srv.helloworldServer)
	book.RegisterBookServer(gr, srv.bookServer)
	blogopenv1.RegisterUserServiceServer(gr, srv.userServer)
	blogopenv1.RegisterArticleServiceServer(gr, srv.articleServer)
	blogopenv1.RegisterCommentServiceServer(gr, srv.commentServer)
}
