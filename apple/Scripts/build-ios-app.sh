#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APPLE_DIR="$ROOT_DIR/apple"
SCHEME="OlcRTCClient iOS"
DESTINATION="${1:-generic/platform=iOS Simulator}"

if [[ -z "${DEVELOPER_DIR:-}" && -d /Applications/Xcode.app/Contents/Developer ]]; then
  export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
fi

if ! xcodebuild -version >/dev/null 2>&1; then
  cat <<'MSG'
Xcode is installed but not ready for iOS builds.

Run these once in Terminal:
  sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
  sudo xcodebuild -license accept

Then rerun:
  ./apple/Scripts/build-ios-app.sh
MSG
  exit 1
fi

if ! xcrun --sdk iphoneos --show-sdk-path >/dev/null 2>&1; then
  cat <<'MSG'
Xcode iOS SDK is not ready.

Run these once in Terminal:
  sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
  sudo xcodebuild -license accept

Then rerun:
  ./apple/Scripts/build-ios-app.sh
MSG
  exit 1
fi

if ! command -v gomobile >/dev/null 2>&1; then
  go install golang.org/x/mobile/cmd/gomobile@latest
fi

gomobile init
"$APPLE_DIR/Scripts/build-xcframework.sh"

if command -v xcodegen >/dev/null 2>&1; then
  (cd "$APPLE_DIR" && xcodegen generate)
fi

xcodebuild \
  -project "$APPLE_DIR/Godwit.xcodeproj" \
  -scheme "$SCHEME" \
  -configuration Debug \
  -destination "$DESTINATION" \
  build

echo "Built iOS app for: $DESTINATION"
