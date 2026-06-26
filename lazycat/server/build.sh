#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
CONTENT_DIR="$SCRIPT_DIR/content"
WEB_DIST_DIR="$ROOT_DIR/web/dist"

rm -rf "$CONTENT_DIR" "$WEB_DIST_DIR"
mkdir -p "$CONTENT_DIR/lazycat-injects" "$WEB_DIST_DIR" "$ROOT_DIR/dist"

if [ ! -f "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js" ]; then
  curl -fsSL "https://developer.lazycat.cloud/lazycat-injects/lzc-file-chooser-inject.js" \
    -o "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js"
fi

cd "$ROOT_DIR/client"
npm ci
VITE_API_BASE_URL="" npm run build
cp -R dist/. "$WEB_DIST_DIR/"
cat > "$WEB_DIST_DIR/README.md" <<'EOF'
This directory is populated by `lazycat/server/build.sh` before compiling the
server binary. The Go `web` package embeds the generated Vite assets from here.
EOF

cd "$ROOT_DIR"
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$CONTENT_DIR/store-server" ./cmd/store-server
