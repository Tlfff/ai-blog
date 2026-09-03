package main

import (
	root "codeup.aliyun.com/qimao/blog/ai-blog/backend/cmd"
	"codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/conf"
)

const serverName = "blog"

func init() {
	if err := conf.InitConfig(serverName); err != nil {
		panic(err)
	}
}

func main() {
	root.Execute()
}
