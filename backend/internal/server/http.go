package server

import (
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/article"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/book"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/comment"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/helloworld"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/like"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/api/user"
	userdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/user"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/middleware"
	"codeup.aliyun.com/qimao/leo/leo/transport/ginhttp"
	gonicgin "github.com/gin-gonic/gin"
)

// NewHTTPServer 创建并聚合全部生成的 HTTP 服务注册器。
//
// 参数说明：
//   - helloworldServer：脚手架示例 HTTP 服务。
//   - bookServer：图书示例 HTTP 服务。
//   - userServer：用户上下文 HTTP 服务。
//   - articleServer：文章上下文 HTTP 服务。
//   - commentServer：评论上下文 HTTP 服务。
//   - likeServer：点赞上下文 HTTP 服务。
//   - sessions：用户会话仓储，用于认证受保护路由。
func NewHTTPServer(helloworldServer helloworld.GreeterHTTPServerController, bookServer book.BookHTTPServerController, userServer user.UserServiceHTTPServerController, articleServer article.ArticleServiceHTTPServerController, commentServer comment.CommentServiceHTTPServerController, likeServer like.LikeServiceHTTPServerController, sessions userdomain.SessionRepository) ginhttp.RegisterServer {
	// 1. 聚合 Controller 并创建共享认证中间件
	if helloworldServer == nil || bookServer == nil || userServer == nil || articleServer == nil || commentServer == nil || likeServer == nil || sessions == nil {
		panic("HTTP 服务注册器缺少必要依赖")
	}
	return &httpServer{
		helloworldServer: helloworldServer,
		bookServer:       bookServer,
		articleServer:    articleServer,
		commentServer:    commentServer,
		likeServer:       likeServer,
		userServer:       userServer,
		userAuth:         middleware.UserAuthMiddleware(sessions),
	}
}

// httpServer 聚合各限界上下文的 HTTP Controller。
type httpServer struct {
	helloworldServer helloworld.GreeterHTTPServerController     // helloworldServer 是脚手架示例服务。
	bookServer       book.BookHTTPServerController              // bookServer 是脚手架图书示例服务。
	articleServer    article.ArticleServiceHTTPServerController // articleServer 是文章上下文 HTTP 服务。
	commentServer    comment.CommentServiceHTTPServerController // commentServer 是评论上下文 HTTP 服务。
	likeServer       like.LikeServiceHTTPServerController       // likeServer 是点赞上下文 HTTP 服务。
	userServer       user.UserServiceHTTPServerController       // userServer 是用户上下文 HTTP 服务。
	userAuth         gonicgin.HandlerFunc                       // userAuth 为受保护路由注入当前用户身份。
}

// Register 将生成的 HTTP 路由注册到 Gin 引擎。
func (srv *httpServer) Register(engine *gonicgin.Engine) {
	// 1. 在路由执行前统一解析需要登录的用户身份
	engine.Use(srv.userAuth)

	// 2. 使用根路由组保持功能文档约定的原始路径
	routerGroup := engine.Group("")
	// 2.1 注册脚手架兼容路由和用户上下文路由
	helloworld.RegisterGreeterHTTPServerController(routerGroup, srv.helloworldServer)
	book.RegisterBookHTTPServerController(routerGroup, srv.bookServer)
	article.RegisterArticleServiceHTTPServerController(routerGroup, srv.articleServer)
	comment.RegisterCommentServiceHTTPServerController(routerGroup, srv.commentServer)
	like.RegisterLikeServiceHTTPServerController(routerGroup, srv.likeServer)
	user.RegisterUserServiceHTTPServerController(routerGroup, srv.userServer)

}
