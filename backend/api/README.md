# `/api`


## 以下提供两种方式接api：

### 1. 在当前项目建立api
* 存放当前服务对外提供接口的定义文件
* 以 `.proto` 为后缀名
* 以 `api` 为包名
* 在当前目录下独立mod，例如 `go get -u codeup.aliyun.com/qimao/blog/ai-blog/backend/api`,
	* 这样做的目的是
		1. 其他包引用只需go get，不需要拷贝proto文件
		2. 以api为独立子包方式，方便管理，依赖项控制到最低。
 
* 使用姿势：
  * 只需要创建添加与helloword同级目录 XXX
  * 业务中当做一个独立的包引用即可


### 2. 在外部仓库建api，本项目只负责引用
* 在 https://codeup.aliyun.com/qimao/freebook-apiclient 仓库中提交自己的api接口，以 `.proto` 为后缀名 
