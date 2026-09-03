GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	Git_Bash= $(subst cmd\,bin\bash.exe,$(dir $(shell where git)))
	INTERNAL_DOMAIN_PROTO_FILES=$(shell $(Git_Bash) -c "find internal/domain -name *.proto")
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
	API_PB_GO_FILES=$(shell $(Git_Bash) -c "find api -name *.pb.go")
else
	INTERNAL_DOMAIN_PROTO_FILES=$(shell find internal/domain -name '*.proto')
	CONF_PROTO_FILES=$(shell find internal/conf -name *.proto)
	API_PROTO_FILES=$(shell find api -name '*.proto')
	API_PB_GO_FILES=$(shell find api -name *.pb.go)
endif

.PHONY: init
# 初始化项目工具和环境
init:
# 安装proto-gen-http等工具
	go install github.com/google/wire/cmd/wire@latest
	go install codeup.aliyun.com/qimao/leo/code-auto/protoc-gen-gin-client-http-leo@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	go install github.com/favadi/protoc-go-inject-tag@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@v0.6.9
	go install go.uber.org/mock/mockgen@latest
	go install github.com/envoyproxy/protoc-gen-validate@latest
	go install mvdan.cc/gofumpt@latest
	go install codeup.aliyun.com/qimao/go-contrib/protoc-gen-go-enum@latest
	go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest

.PHONY: config
# 生成配置文件和枚举
config:
	protoc --proto_path=./internal \
	       --go-enum_out=paths=source_relative:./internal \
	       --go-enum_opt=paths=source_relative \
	       --doc_out=./docs --doc_opt=docs/templates/const.tmpl,const.md \
	       $(INTERNAL_DOMAIN_PROTO_FILES)
	protoc --proto_path=./internal \
    	       --proto_path=./third_party \
     	       --go_out=paths=source_relative:./internal \
    	       $(CONF_PROTO_FILES)

.PHONY: api
# 由 proto 生成接口层代码
api:
	protoc --proto_path=./api \
		   --proto_path=./third_party \
		   --go_out=paths=source_relative:./api \
		   --gin-client-http-leo_out=paths=source_relative:./api \
		   --go-grpc_out=. \
		   --go-grpc_opt=module=codeup.aliyun.com/qimao/blog/ai-blog/backend \
		   --openapi_out=fq_schema_naming=true,naming=proto,default_response=false:. \
		   --validate_out=paths=source_relative,lang=go:./api \
		   $(API_PROTO_FILES)
	@find . -name '*.pb.go' |grep -v vendor | xargs -n1 -IX bash -c 'sed s/,omitempty// X > X.tmp && mv X{.tmp,}'
	@@make fmt


.PHONY: inject-tag
# inject-tag 使用 protocol file 注释覆盖 struct 的tag
inject-tag:
	@ls $(API_PB_GO_FILES) | xargs -n1 -I {} protoc-go-inject-tag -input={}


# fmt 格式化代码
fmt:
	@go fmt ./...

# lint 代码检查
lint:
	@make fmt
	golangci-lint run  ./...

.PHONY: build
# build 编译
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...


.PHONY: all
# 重新生产代码并编译
all:
	make api;
	make config;
	make generate;

mysql_model:
	reverse -f sql_reverse.yaml

.PHONY: gen
# generate 缩写
gen:
	make generate

#文件存在, 则
ifneq ($(wildcard ./go.work),)
.PHONY: generate
# generate
generate:
	go mod tidy
	GOFLAGS="-mod=readonly" go generate ./...
	make fmt
else
.PHONY: generate
# generate
generate:
	go mod tidy
	GOFLAGS="-mod=mod" go generate ./...
	make fmt
endif

# run 本地编译 运行
run:
	@go run main.go http

runjob:
	@go run main.go job blog
