# `/internal`

### 应用层

* 主要业务 ，service服务入口，domain层的实现 存放各个领域的抽象业务代码

```

├── internal 
│   ├── app 应用层
│   │   ├── consumer 消费者应用
│   │   ├── job 脚本应用
│   │   ├── service 接口服务 （服务聚合）
│   │   │   └── helloworld
│	├── domain 领域服务
│	│   └── repo
│	│       ├── entity 实体类
│	│       │   └── vo 实体类中聚合类
│	│       ├── meta 常量
│	│       ├── mysql_repository mysql仓库层
│	│       │   └── factory do <-> po
│	│       └── redis_repository redis仓库层
│	│           └── factory redis没有po  
│   ├── assembly 依赖注入编写和生成文件目录
│   ├── clients 中间件客户端启动目录（注入用）
│   ├── middleware 中间件
│   ├── model 第三方中间件的po
│   │   ├── mysql
│   │   └── redis
│   ├── pkg 工具包
│   │   ├── async
│   │   └── utils
│   └── server http服务注册，提供给wire注入使用
```
