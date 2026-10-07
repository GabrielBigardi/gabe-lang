#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
go run ./tools/checkfmt
go vet ./...
GABE_REQUIRE_NATIVE_TESTS=1 go test -count=1 ./...
go build -trimpath -o bin/ ./cmd/gabe
