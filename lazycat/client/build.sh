#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
CONTENT_DIR="$SCRIPT_DIR/content"
EMBED_DIST_DIR="$ROOT_DIR/clientembed/dist"

rm -rf "$CONTENT_DIR" "$EMBED_DIST_DIR"
mkdir -p "$CONTENT_DIR/lazycat-injects" "$EMBED_DIST_DIR" "$ROOT_DIR/dist"

curl -fsSL "https://developer.lazycat.cloud/lazycat-injects/lzc-file-chooser-inject.js" \
  -o "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js"

cd "$ROOT_DIR/client"
npm ci
VITE_API_BASE_URL="${CLIENT_API_BASE_URL:-}" npm run build
cp -R dist/. "$EMBED_DIST_DIR/"

EMBED_DIST_DIR="$EMBED_DIST_DIR" node <<'NODE'
const fs = require('node:fs');
const path = require('node:path');

const config = {
  apiBaseURL: process.env.CLIENT_API_BASE_URL || '',
  defaultSourceURL: process.env.CLIENT_DEFAULT_SOURCE_URL || '',
  defaultSourceName: process.env.CLIENT_DEFAULT_SOURCE_NAME || 'Community Store',
};

fs.writeFileSync(
  path.join(process.env.EMBED_DIST_DIR, 'app-config.js'),
  `window.LAZYCAT_APPSTORE_CONFIG = ${JSON.stringify(config, null, 2)};\n`,
);
NODE

cd "$ROOT_DIR"
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$CONTENT_DIR/store-client" ./cmd/store-client
