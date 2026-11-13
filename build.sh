#!/usr/bin/env sh
# Build RigStart.exe for Windows (works from Linux, macOS or Windows with Go 1.22+).
set -e
cd "$(dirname "$0")"
go test ./...
# Embed the icon in the .exe (requires: go install github.com/akavel/rsrc@v0.10.2)
if command -v rsrc >/dev/null 2>&1; then
  rsrc -ico ui/icon.ico -arch amd64 -o rsrc_windows_amd64.syso
fi
mkdir -p dist
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H=windowsgui -s -w" -o dist/RigStart.exe .
echo "Built dist/RigStart.exe"
