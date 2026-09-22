#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXT_BIN="$ROOT/extensions/vscode/bin"
mkdir -p "$EXT_BIN/linux-x64" "$EXT_BIN/win32-x64" "$EXT_BIN/darwin-arm64"

GOOS=linux GOARCH=amd64 go build -o "$EXT_BIN/linux-x64/dre-replay" "$ROOT/dre-replay-cli/cmd/dre-replay"
GOOS=windows GOARCH=amd64 go build -o "$EXT_BIN/win32-x64/dre-replay.exe" "$ROOT/dre-replay-cli/cmd/dre-replay"
GOOS=darwin GOARCH=arm64 go build -o "$EXT_BIN/darwin-arm64/dre-replay" "$ROOT/dre-replay-cli/cmd/dre-replay"

GOOS=linux GOARCH=amd64 go build -o "$EXT_BIN/linux-x64/bugit" "$ROOT/bugit-cli/cmd/bugit"
GOOS=windows GOARCH=amd64 go build -o "$EXT_BIN/win32-x64/bugit.exe" "$ROOT/bugit-cli/cmd/bugit"
GOOS=darwin GOARCH=arm64 go build -o "$EXT_BIN/darwin-arm64/bugit" "$ROOT/bugit-cli/cmd/bugit"

echo "Bundled bugit + dre-replay into extensions/vscode/bin/"
