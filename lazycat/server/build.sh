#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
CONTENT_DIR="$SCRIPT_DIR/content"
WEB_DIST_DIR="$ROOT_DIR/web/dist"
PACKAGE_VERSION=$(awk '/^version:/ { print $2; exit }' "$SCRIPT_DIR/package.yml")
CLIENT_PACKAGE_VERSION=$(awk '/^version:/ { print $2; exit }' "$ROOT_DIR/lazycat/client/package.yml")

rm -rf "$CONTENT_DIR" "$WEB_DIST_DIR"
mkdir -p "$CONTENT_DIR/lazycat-injects" "$WEB_DIST_DIR" "$ROOT_DIR/dist"

if [ ! -f "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js" ]; then
  curl -fsSL "https://developer.lazycat.cloud/lazycat-injects/lzc-file-chooser-inject.js" \
    -o "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js"
fi

cd "$ROOT_DIR/client"
npm ci
VITE_API_BASE_URL="." npm run build
cp -R dist/. "$WEB_DIST_DIR/"
cat > "$WEB_DIST_DIR/README.md" <<'EOF'
This directory is populated by `lazycat/server/build.sh` before compiling the
server binary. The Go `web` package embeds the generated Vite assets from here.
EOF

cd "$ROOT_DIR"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X lazycat.community/appstore/internal/buildinfo.Version=$PACKAGE_VERSION -X lazycat.community/appstore/internal/buildinfo.ClientVersion=$CLIENT_PACKAGE_VERSION" -o "$CONTENT_DIR/store-server" ./cmd/store-server
