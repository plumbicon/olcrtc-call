#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APPLE_DIR="$ROOT_DIR/apple"
SCHEME="OlcRTCClient iOS"
CONFIGURATION="${CONFIGURATION:-Release}"
BUILD_DIR="${BUILD_DIR:-$APPLE_DIR/.build/ios-unsigned-local}"
PAYLOAD_DIR="$BUILD_DIR/Payload"
IPA_PATH="$BUILD_DIR/Godwit-unsigned-local.ipa"

if [[ -z "${DEVELOPER_DIR:-}" && -d /Applications/Xcode.app/Contents/Developer ]]; then
  export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
fi

if ! xcodebuild -version >/dev/null 2>&1; then
  cat <<'MSG'
Xcode is required for iOS IPA builds.

Run these once in Terminal:
  sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
  sudo xcodebuild -license accept

Then rerun:
  ./apple/Scripts/build-ios-unsigned-local-ipa.sh
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

rm -rf "$BUILD_DIR"
mkdir -p "$PAYLOAD_DIR"

xcodebuild \
  -project "$APPLE_DIR/Godwit.xcodeproj" \
  -scheme "$SCHEME" \
  -configuration "$CONFIGURATION" \
  -sdk iphoneos \
  -destination "generic/platform=iOS" \
  -derivedDataPath "$BUILD_DIR/DerivedData" \
  SWIFT_ACTIVE_COMPILATION_CONDITIONS=LOCAL_SOCKS_ONLY \
  CODE_SIGNING_ALLOWED=NO \
  CODE_SIGNING_REQUIRED=NO \
  CODE_SIGN_IDENTITY="" \
  build

APP_PATH="$(find "$BUILD_DIR/DerivedData/Build/Products/$CONFIGURATION-iphoneos" -maxdepth 1 -name '*.app' -print -quit)"
if [[ -z "$APP_PATH" ]]; then
  echo "Build finished, but no .app was found."
  exit 1
fi

cp -R "$APP_PATH" "$PAYLOAD_DIR/"
APP_BUNDLE="$PAYLOAD_DIR/$(basename "$APP_PATH")"

# This local-SOCKS package is intended for post-build signing without the
# Network Extension entitlement. Keep the main app and drop the packet tunnel.
rm -rf "$APP_BUNDLE/PlugIns"

rm -f "$IPA_PATH"
(cd "$BUILD_DIR" && /usr/bin/zip -qry "$IPA_PATH" Payload)

echo "Built unsigned local IPA:"
echo "  $IPA_PATH"
echo
echo "Note: this package is LOCAL_SOCKS_ONLY. It does not include the iOS Packet Tunnel extension,"
echo "so it cannot route other iOS apps through VPN. For real device traffic routing, build a"
echo "signed IPA with DEVELOPMENT_TEAM using Scripts/build-ios-ipa.sh."
