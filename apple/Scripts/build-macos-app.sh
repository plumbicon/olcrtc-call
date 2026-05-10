#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APPLE_DIR="$ROOT_DIR/apple"
CONFIGURATION="${CONFIGURATION:-debug}"
APP_DIR="$APPLE_DIR/.build/olcRTC.app"
CONTENTS_DIR="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"

"$APPLE_DIR/Scripts/build-macos-cli.sh"

cd "$APPLE_DIR"
swift build -c "$CONFIGURATION" --product OlcRTCClientMac

SWIFT_BINARY="$APPLE_DIR/.build/arm64-apple-macosx/$CONFIGURATION/OlcRTCClientMac"
if [ ! -x "$SWIFT_BINARY" ]; then
  SWIFT_BINARY="$APPLE_DIR/.build/$CONFIGURATION/OlcRTCClientMac"
fi

rm -rf "$APP_DIR"
mkdir -p "$MACOS_DIR" "$RESOURCES_DIR"

cp "$SWIFT_BINARY" "$MACOS_DIR/olcRTC"
cp "$APPLE_DIR/.build/olcrtc-macos" "$RESOURCES_DIR/olcrtc-macos"
cp -R "$ROOT_DIR/data" "$RESOURCES_DIR/data"

cat > "$CONTENTS_DIR/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleExecutable</key>
  <string>olcRTC</string>
  <key>CFBundleIdentifier</key>
  <string>community.openlibre.olcrtc.macos.dev</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>olcRTC</string>
  <key>CFBundleDisplayName</key>
  <string>olcRTC</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>0.1.0</string>
  <key>CFBundleVersion</key>
  <string>1</string>
  <key>LSMinimumSystemVersion</key>
  <string>13.0</string>
  <key>LSApplicationCategoryType</key>
  <string>public.app-category.utilities</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

chmod +x "$MACOS_DIR/olcRTC" "$RESOURCES_DIR/olcrtc-macos"

if command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "$APP_DIR" >/dev/null
fi

echo "Built $APP_DIR"
echo "Run it with:"
echo "  open \"$APP_DIR\""
