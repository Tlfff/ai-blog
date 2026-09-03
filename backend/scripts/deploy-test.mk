# build 测试编译
.PHONY: deploy-build-test
deploy-build-test:
	go build -o bin/main -v -ldflags '-w -s' -tags=musl main.go
