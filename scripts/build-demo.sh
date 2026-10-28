#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
GOOS=js GOARCH=wasm go build -trimpath -ldflags='-s -w' -o docs/demo.wasm ./cmd/demo
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" docs/wasm_exec.js
