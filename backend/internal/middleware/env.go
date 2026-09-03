package middleware

// var noneCheckUrls = []string{"/check/env", "/check/auths"}
//
// // EnvMiddleware 前后端环境检测 noneCheckUrls 不检查 env 的 path
// func EnvMiddleware() middleware.Middleware {
// 	return func(handler middleware.Handler) middleware.Handler {
// 		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
// 			if tr, ok := transport.FromServerContext(ctx); ok {
// 				if ht, ok := tr.(*http.Transport); ok {
// 					path := ht.Request().URL.Path
// 					if !utils.Include(noneCheckUrls, path) {
// 						query := ht.Request().URL.Query()
// 						envFe := query.Get("env")
// 						environment := "0"
// 						if env.IsReleasing() {
// 							environment = "1"
// 						}
// 						if envFe == "" || (envFe != "" && envFe != environment) {
// 							return reply, errors.New("环境异常！请检查当前环境")
// 						}
// 					}
// 				}
// 			}
// 			reply, err = handler(ctx, req)
// 			return reply, err
// 		}
// 	}
// }
