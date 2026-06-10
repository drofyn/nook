#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
TAG="${NOOK_TAG:-${GITHUB_REF_NAME:-v0.1.0}}"
NOOK_VERSION="${NOOK_VERSION:-${TAG#v}}"
NOOK_RELEASE="${NOOK_RELEASE:-1}"
IPK_OUT="$DIST_DIR/nook_${NOOK_VERSION}-${NOOK_RELEASE}_aarch64_cortex-a53.ipk"
APK_OUT="$DIST_DIR/nook_${NOOK_VERSION}-r${NOOK_RELEASE}_aarch64_cortex-a53.apk"
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

sh "$ROOT_DIR/scripts/build-openwrt-arm64.sh"

run_nfpm package -f "$ROOT_DIR/nfpm.yaml" -p ipk -t "$IPK_OUT"
run_nfpm package -f "$ROOT_DIR/nfpm.yaml" -p apk -t "$APK_OUT"

ls -lh "$IPK_OUT" "$APK_OUT"
