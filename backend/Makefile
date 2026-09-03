
# 默认 help
.DEFAULT_GOAL := help

# 项目通用变量
GIT_TAG=$(shell git tag | grep "v*.[0-9]$$" | sort -r --version-sort | head -n1)
QM_APP_GIT_COMMIT_ID=$(shell git rev-parse --short=8 HEAD)
GIT_TAG_COMMIT = $(GIT_TAG)-$(QM_APP_GIT_COMMIT_ID)

# 项目镜像标签 自定义环境变量一定要以USER_开头
$(shell echo "USER_APP_VERSION=$(GIT_TAG_COMMIT)" > .env)
# 项目环境分组 自定义环境变量一定要以USER_开头
$(shell echo "USER_ENV_GROUP=$(ENV_GROUP)" >> .env)

# 本地开发相关
include ./scripts/develop.mk
include ./scripts/deploy-dev.mk
include ./scripts/deploy-test.mk
include ./scripts/deploy-prod.mk
