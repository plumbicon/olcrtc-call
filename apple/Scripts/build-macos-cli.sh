#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$ROOT_DIR/apple/.build/olcrtc-macos"

mkdir -p "$(dirname "$OUT")"
cd "$ROOT_DIR"

go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "$OUT" \
  ./cmd/olcrtc

echo "Built $OUT"
