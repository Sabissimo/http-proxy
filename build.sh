#!/usr/bin/env bash
# Produces httpproxy.exe. Run from Git Bash.
set -euo pipefail
cd "$(dirname "$0")"

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags="-s -w" -o httpproxy.exe ./cmd/httpproxy
echo "built httpproxy.exe"
