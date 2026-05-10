#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APPLE_DIR="$ROOT_DIR/apple"

if [[ -z "${DEVELOPER_DIR:-}" && -d /Applications/Xcode.app/Contents/Developer ]]; then
  export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
fi

if ! xcodebuild -version >/dev/null 2>&1; then
  cat <<'MSG'
Xcode is required for iOS/macOS app builds.

Install Xcode from the App Store or Apple Developer Downloads. If it is already
installed, finish setup from Terminal:
  sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
  sudo xcodebuild -license accept

After that, rerun this script.
MSG
  exit 1
fi

if ! xcrun --sdk iphoneos --show-sdk-path >/dev/null 2>&1; then
  cat <<'MSG'
Xcode is installed, but the iOS SDK is not ready yet.

Run these once in Terminal:
  sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
  sudo xcodebuild -license accept

After that, rerun this script.
MSG
  exit 1
fi

if ! command -v gomobile >/dev/null 2>&1; then
  go install golang.org/x/mobile/cmd/gomobile@latest
fi

gomobile init
"$APPLE_DIR/Scripts/build-xcframework.sh"

if ! command -v xcodegen >/dev/null 2>&1; then
  if command -v brew >/dev/null 2>&1; then
    brew install xcodegen
  else
    echo "xcodegen is not installed. Install it, then run: cd apple && xcodegen generate"
    exit 1
  fi
fi

cd "$APPLE_DIR"
xcodegen generate

echo "Ready: $APPLE_DIR/OlcRTCClient.xcodeproj"
