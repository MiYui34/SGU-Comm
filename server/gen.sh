#!/bin/sh
# Regenerates the Go protobuf code from the Mumble proto files.
# Needs protoc and protoc-gen-go (go install google.golang.org/protobuf/cmd/protoc-gen-go@latest).
set -e
cd "$(dirname "$0")"
PATH="$PATH:$(go env GOPATH)/bin" protoc -I ../packages/protocol/proto \
  --go_out=internal/mumble/pb --go_opt=paths=source_relative \
  --go_opt=MMumble.proto=kallisti.chat/server/internal/mumble/pb \
  Mumble.proto
