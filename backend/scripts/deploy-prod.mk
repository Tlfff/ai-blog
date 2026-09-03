# build 生产编译
.PHONY: deploy-build-prod
deploy-build-prod:
	go build -o bin/main -v -a -ldflags '-w -s' -tags=musl main.go
