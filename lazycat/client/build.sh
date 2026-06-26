#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
CONTENT_DIR="$SCRIPT_DIR/content"

rm -rf "$CONTENT_DIR"
mkdir -p "$CONTENT_DIR/lazycat-injects" "$ROOT_DIR/dist"

curl -fsSL "https://developer.lazycat.cloud/lazycat-injects/lzc-file-chooser-inject.js" \
  -o "$CONTENT_DIR/lazycat-injects/lzc-file-chooser-inject.js"

cd "$ROOT_DIR/client"
npm ci
VITE_API_BASE_URL="${CLIENT_API_BASE_URL:-}" npm run build
cp -R dist "$CONTENT_DIR/dist"

CONTENT_DIR="$CONTENT_DIR" node <<'NODE'
const fs = require('node:fs');
const path = require('node:path');

const config = {
  apiBaseURL: process.env.CLIENT_API_BASE_URL || '',
  defaultSourceURL: process.env.CLIENT_DEFAULT_SOURCE_URL || '',
  defaultSourceName: process.env.CLIENT_DEFAULT_SOURCE_NAME || 'Community Store',
};

fs.writeFileSync(
  path.join(process.env.CONTENT_DIR, 'dist', 'app-config.js'),
  `window.LAZYCAT_APPSTORE_CONFIG = ${JSON.stringify(config, null, 2)};\n`,
);
NODE
