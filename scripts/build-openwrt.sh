#!/bin/sh
set -eu

ARCH="${1:-}"
case "$ARCH" in
  arm64)
    GOARCH=arm64
    NOOK_ARCH=aarch64_generic
    ;;
  x86_64)
    GOARCH=amd64
    NOOK_ARCH=x86_64
    ;;
  *)
    echo "usage: $0 {arm64|x86_64}" >&2
    exit 1
    ;;
esac

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
OUT_DIR="$ROOT_DIR/dist/openwrt-${NOOK_ARCH}"
OUT="$OUT_DIR/nook"
NOOK_VERSION="${NOOK_VERSION:-dev}"

mkdir -p "$OUT_DIR"

cd "$ROOT_DIR"
CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build \
  -trimpath \
  -ldflags="-s -w -X main.version=$NOOK_VERSION" \
  -o "$OUT" \
  .

chmod 755 "$OUT"

printf 'built: %s\n' "$OUT"
ls -lh "$OUT"
file "$OUT" 2>/dev/null || true
