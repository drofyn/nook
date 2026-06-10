#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
OUT_DIR="$ROOT_DIR/dist/openwrt-aarch64_cortex-a53"
OUT="$OUT_DIR/nook"
NOOK_VERSION="${NOOK_VERSION:-dev}"

mkdir -p "$OUT_DIR"

cd "$ROOT_DIR"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -trimpath \
  -ldflags="-s -w -X main.version=$NOOK_VERSION" \
  -o "$OUT" \
  .

chmod 755 "$OUT"

printf 'built: %s\n' "$OUT"
ls -lh "$OUT"
file "$OUT" 2>/dev/null || true
