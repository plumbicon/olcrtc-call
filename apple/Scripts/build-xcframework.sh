#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="$ROOT_DIR/apple/Frameworks"
OUT="$OUT_DIR/Mobile.xcframework"

if [[ -z "${DEVELOPER_DIR:-}" && -d /Applications/Xcode.app/Contents/Developer ]]; then
  export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
fi

command -v gomobile >/dev/null 2>&1 || {
  echo "gomobile not found. Install it with:"
  echo "  go install golang.org/x/mobile/cmd/gomobile@latest"
  echo "  gomobile init"
  exit 1
}

if ! xcrun --sdk iphoneos --show-sdk-path >/dev/null 2>&1; then
  cat <<'MSG'
Xcode iOS SDK is not ready.

If Xcode is installed, finish the first-run setup from Terminal:
  sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
  sudo xcodebuild -license accept

Then rerun this script.
MSG
  exit 1
fi

mkdir -p "$OUT_DIR"
rm -rf "$OUT"

cd "$ROOT_DIR"

gomobile bind \
  -target=ios,iossimulator,macos \
  -ldflags="-s -w -checklinkname=0" \
  -o "$OUT" \
  ./mobile

echo "Built $OUT"
