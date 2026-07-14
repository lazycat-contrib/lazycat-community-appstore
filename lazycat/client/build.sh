#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
CONTENT_DIR="$SCRIPT_DIR/content"
EMBED_DIST_DIR="$ROOT_DIR/clientembed/dist"
PACKAGE_VERSION=$(awk '/^version:/ { print $2; exit }' "$SCRIPT_DIR/package.yml")

rm -rf "$CONTENT_DIR" "$EMBED_DIST_DIR"
mkdir -p "$CONTENT_DIR/lazycat-injects" "$EMBED_DIST_DIR" "$ROOT_DIR/dist"

curl -fsSL "https://developer.lazycat.cloud/lazycat-injects/lzc-file-chooser-inject.js" \
  -o "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js"

cd "$ROOT_DIR/client"
npm ci
# Client-specific defaults are written to app-config.js below.
npm run build
cp -R dist/. "$EMBED_DIST_DIR/"

CLIENT_APP_VERSION="$PACKAGE_VERSION" EMBED_DIST_DIR="$EMBED_DIST_DIR" node <<'NODE'
const fs = require('node:fs');
const path = require('node:path');

const config = {
  apiBaseURL: process.env.CLIENT_API_BASE_URL || '',
  defaultSourceURL: process.env.CLIENT_DEFAULT_SOURCE_URL || '',
  defaultSourceName: process.env.CLIENT_DEFAULT_SOURCE_NAME || '喵喵私有商店',
  appVersion: process.env.CLIENT_APP_VERSION || '',
};

fs.writeFileSync(
  path.join(process.env.EMBED_DIST_DIR, 'app-config.js'),
  `window.LAZYCAT_APPSTORE_CONFIG = ${JSON.stringify(config, null, 2)};\n`,
);
NODE

cd "$ROOT_DIR"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X lazycat.community/appstore/internal/buildinfo.Version=$PACKAGE_VERSION" -o "$CONTENT_DIR/store-client" ./cmd/store-client
