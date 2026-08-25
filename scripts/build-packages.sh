#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
TAG="${NOOK_TAG:-${GITHUB_REF_NAME:-v0.1.0}}"
NOOK_VERSION="${NOOK_VERSION:-${TAG#v}}"
NOOK_RELEASE="${NOOK_RELEASE:-1}"
NFPM="${NFPM:-nfpm}"

export NOOK_VERSION NOOK_RELEASE

if [ "$NFPM" = "nfpm" ] && ! command -v nfpm >/dev/null 2>&1; then
  echo "nfpm is required to build packages" >&2
  echo "Install it from https://nfpm.goreleaser.com/docs/install/" >&2
  exit 1
fi

run_nfpm() {
  if [ "$NFPM" = "nfpm" ]; then
    nfpm "$@"
  else
    sh -c "$NFPM \"\$@\"" sh "$@"
  fi
}

case "$TAG" in
  v[0-9]*) ;;
  *)
    echo "NOOK_TAG or GITHUB_REF_NAME must be a v-prefixed version tag" >&2
    exit 1
    ;;
esac

rm -rf "$DIST_DIR"

build_arch() {
  arch_token="$1"
  goarch="$2"
  nook_arch="$3"

  sh "$ROOT_DIR/scripts/build-openwrt.sh" "$arch_token"

  config_out="$DIST_DIR/.nfpm-${nook_arch}.yaml"
  sed \
    -e "s|\${NOOK_GOARCH}|${goarch}|g" \
    -e "s|\${NOOK_ARCH}|${nook_arch}|g" \
    -e "s|\${NOOK_VERSION}|${NOOK_VERSION}|g" \
    -e "s|\${NOOK_RELEASE}|${NOOK_RELEASE}|g" \
    "$ROOT_DIR/nfpm.yaml" > "$config_out"

  ipk_out="$DIST_DIR/nook_${NOOK_VERSION}-${NOOK_RELEASE}_${nook_arch}.ipk"
  apk_out="$DIST_DIR/nook_${NOOK_VERSION}-r${NOOK_RELEASE}_${nook_arch}.apk"

  run_nfpm package -f "$config_out" -p ipk -t "$ipk_out"
  run_nfpm package -f "$config_out" -p apk -t "$apk_out"

  rm -f "$config_out"

  ls -lh "$ipk_out" "$apk_out"
}

build_arch arm64  arm64  aarch64_cortex-a53
build_arch x86_64 amd64  x86_64

ls -lh "$DIST_DIR"/*.ipk "$DIST_DIR"/*.apk
