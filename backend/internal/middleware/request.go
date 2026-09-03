package middleware

import (
	"context"
	"net"
	nethttp "net/http"
	"strings"

	"codeup.aliyun.com/qimao/leo/leo/log"
	ginctxmdw "codeup.aliyun.com/qimao/leo/leo/middleware/ginhttp/context"
	ginrequestid "codeup.aliyun.com/qimao/leo/leo/middleware/ginhttp/requestid"
	"github.com/gin-gonic/gin"
)

// CtxMiddleware 请求Ctx Middleware
func CtxMiddleware(requestIdKey string) gin.HandlerFunc {
	// requestIdKey 网关中塞入的请求ID的key，默认Request-ID
	if len(requestIdKey) == 0 {
		// if requestIdKey ==  "" {
		requestIdKey = "Request-ID"
	}
	return ginctxmdw.Middleware(
		log.NewContextClosure(
			log.L(),
			func(ctx context.Context) []log.Field {
				requestID, _ := ginrequestid.FromContext(ctx)
				return []log.Field{log.F{K: requestIdKey, V: requestID}}
			},
		),
	)
}

// func HeaderMiddleware() middleware.Middleware {
// 	return func(handler middleware.Handler) middleware.Handler {
// 		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
// 			if tr, ok := transport.FromServerContext(ctx); ok {
// 				if ht, ok := tr.(*http.Transport); ok {
// 					md := make(metadata.Metadata)
// 					if mdd, ok := metadata.FromClientContext(ctx); ok {
// 						md = mdd.Clone()
// 					}
// 					md["client_ip"] = clientIP(ht.Request())
// 					md["endpoint"] = getFirstIpAddr(ht.Endpoint())
// 					md["oper_url"] = ht.Request().RequestURI
// 					md["method"] = ht.Request().Method
// 					md["ua"] = ht.Request().UserAgent()
// 					ctx = metadata.NewClientContext(ctx, md)
// 				}
// 			}
// 			reply, err = handler(ctx, req)
// 			return reply, err
// 		}
// 	}
// }

func clientIP(r *nethttp.Request) string {
	var clientIP string
	clientIP = getFirstIpAddr(r.Header.Get("X-Real-Ip"))
	if len(clientIP) > 0 {
		return clientIP
	}

	clientIP = getFirstIpAddr(r.Header.Get("X-Forwarded-For"))
	if len(clientIP) > 0 {
		return clientIP
	}

	clientIP = getFirstIpAddr(r.RemoteAddr)
	if len(clientIP) > 6 {
		return clientIP
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(r.Host)); err == nil {
		return ip
	}
	return ""
}

func getFirstIpAddr(clientIP string) string {
	if index := strings.IndexByte(clientIP, ','); index >= 0 {
		clientIP = clientIP[0:index]
	}
	clientIP = strings.TrimSpace(clientIP)
	if len(clientIP) > 0 {
		return clientIP
	}
	return ""
}
