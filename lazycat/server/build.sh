#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
CONTENT_DIR="$SCRIPT_DIR/content"
EMBED_DIST_DIR="$ROOT_DIR/clientembed/dist"
PACKAGE_VERSION=$(awk '/^version:/ { print $2; exit }' "$SCRIPT_DIR/package.yml")

rm -rf "$CONTENT_DIR" "$EMBED_DIST_DIR"
mkdir -p "$CONTENT_DIR/lazycat-injects" "$EMBED_DIST_DIR" "$ROOT_DIR/dist"

if [ ! -f "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js" ]; then
  curl -fsSL "https://developer.lazycat.cloud/lazycat-injects/lzc-file-chooser-inject.js" \
    -o "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js"
fi

cd "$ROOT_DIR/client"
npm ci
# The server injects its API origin through /app-config.js at runtime.
npm run build
cp -R dist/. "$EMBED_DIST_DIR/"

cd "$ROOT_DIR"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X lazycat.community/appstore/internal/buildinfo.Version=$PACKAGE_VERSION" -o "$CONTENT_DIR/store-server" ./cmd/store-server
