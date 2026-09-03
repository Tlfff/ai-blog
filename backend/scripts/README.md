# `/scripts`

* 存放多个环境的Makefile， 和dockerfile 以及其他脚本文件

## 目录结构

* makefile 分为4个文件，分别表示3个环境的命令，和一个本地开发的命令

1. 开发环境：     [deploy-dev.mk](./deploy-dev.mk)
2. 生产环境：     [deploy-prod.mk](./deploy-prod.mk)
3. 测试环境：     [deploy-test.mk](./deploy-test.mk)
4. 本地环境：     [develop.mk](./develop.mk)
 
