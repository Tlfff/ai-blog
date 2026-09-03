# build dev编译
.PHONY: deploy-build-dev
deploy-build-test:
	go build -o bin/main -v -ldflags '-w -s' -tags=musl main.go
