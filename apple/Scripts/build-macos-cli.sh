#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APPLE_DIR="$ROOT_DIR/apple"
OLCRTC_DIR="$ROOT_DIR/olcrtc"
OUT="$APPLE_DIR/.build/olcrtc-macos"

mkdir -p "$(dirname "$OUT")"
cd "$OLCRTC_DIR"

go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "$OUT" \
  ./cmd/olcrtc

echo "Built $OUT"
