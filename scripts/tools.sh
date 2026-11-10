#!/usr/bin/env sh
# Installs the generators and linters into the given directory at the versions
# the Makefile exports, skipping any that is already there at that version.
set -eu

bin="$1"
mkdir -p "$bin"

installed() {
  [ -x "$bin/$1" ] && "$bin/$1" "$2" 2>/dev/null | grep -q "$3"
}

if ! installed buf --version "${BUF_VERSION#v}"; then
  GOBIN="$bin" go install "github.com/bufbuild/buf/cmd/buf@$BUF_VERSION"
fi
if ! installed protoc-gen-go --version "$PROTOC_GEN_GO_VERSION"; then
  GOBIN="$bin" go install "google.golang.org/protobuf/cmd/protoc-gen-go@$PROTOC_GEN_GO_VERSION"
fi
if ! installed golangci-lint version "${GOLANGCI_LINT_VERSION#v}"; then
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
    | sh -s -- -b "$bin" "$GOLANGCI_LINT_VERSION"
fi

for tool in buf protoc-gen-go golangci-lint; do
  printf '%-16s %s\n' "$tool" "$("$bin/$tool" --version 2>/dev/null || "$bin/$tool" version)"
done
