#!/bin/zsh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
BIN="$ROOT/.tools/bin"
mkdir -p "$BIN"

export GOBIN="$BIN"
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

copy_pinned() {
  local name="$1"
  local expected="$2"
  local source
  source="$(command -v "$name" || true)"
  if [[ -z "$source" ]]; then
    echo "missing required generator: $name" >&2
    exit 1
  fi
  if ! go version -m "$source" 2>/dev/null | grep -Fq "$expected"; then
    echo "$name does not match pinned build metadata: $expected" >&2
    exit 1
  fi
  cp "$source" "$BIN/$name"
  chmod +x "$BIN/$name"
}

copy_pinned protoc-gen-gin-client-http-leo $'mod\tcodeup.aliyun.com/qimao/leo/code-auto/protoc-gen-gin-client-http-leo\tv0.3.0'
copy_pinned protoc-gen-openapi $'mod\tgithub.com/google/gnostic\tv0.6.9'
copy_pinned protoc-gen-validate $'mod\tgithub.com/envoyproxy/protoc-gen-validate\tv1.2.1'
copy_pinned protoc-gen-go-enum $'mod\tcodeup.aliyun.com/qimao/go-contrib/protoc-gen-go-enum\tv0.0.0-20240806123052-88a181a147bd'
copy_pinned protoc-gen-doc $'mod\tgithub.com/pseudomuto/protoc-gen-doc\tv1.5.1'
copy_pinned wire $'mod\tgithub.com/google/wire\tv0.7.0'

echo "CrewOps generator toolchain installed in $BIN"
