#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APPLE_DIR="$ROOT_DIR/apple"
SCHEME="OlcRTCClient iOS"
CONFIGURATION="${CONFIGURATION:-Release}"
EXPORT_METHOD="${EXPORT_METHOD:-development}"
ARCHIVE_DIR="${ARCHIVE_DIR:-$APPLE_DIR/.build/ios-archive}"
EXPORT_DIR="${EXPORT_DIR:-$APPLE_DIR/.build/ios-ipa}"
ARCHIVE_PATH="$ARCHIVE_DIR/Godwit.xcarchive"
EXPORT_OPTIONS="$ARCHIVE_DIR/ExportOptions.plist"

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
  ./apple/Scripts/build-ios-ipa.sh
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
  ./apple/Scripts/build-ios-ipa.sh
MSG
  exit 1
fi

if [[ -z "${DEVELOPMENT_TEAM:-}" ]]; then
  cat <<'MSG'
DEVELOPMENT_TEAM is required for an installable IPA.

Example:
  DEVELOPMENT_TEAM=ABCDE12345 EXPORT_METHOD=development ./apple/Scripts/build-ios-ipa.sh

The app and packet tunnel extension provisioning profiles must both include the
Network Extension packet-tunnel-provider entitlement.
MSG
  exit 1
fi

case "$EXPORT_METHOD" in
  development|ad-hoc|app-store|enterprise)
    ;;
  *)
    echo "Unsupported EXPORT_METHOD=$EXPORT_METHOD"
    echo "Use one of: development, ad-hoc, app-store, enterprise"
    exit 1
    ;;
esac

if ! command -v gomobile >/dev/null 2>&1; then
  go install golang.org/x/mobile/cmd/gomobile@latest
fi

gomobile init
"$APPLE_DIR/Scripts/build-xcframework.sh"

if command -v xcodegen >/dev/null 2>&1; then
  (cd "$APPLE_DIR" && xcodegen generate)
fi

rm -rf "$ARCHIVE_DIR" "$EXPORT_DIR"
mkdir -p "$ARCHIVE_DIR" "$EXPORT_DIR"

xcodebuild \
  -project "$APPLE_DIR/Godwit.xcodeproj" \
  -scheme "$SCHEME" \
  -configuration "$CONFIGURATION" \
  -destination "generic/platform=iOS" \
  -archivePath "$ARCHIVE_PATH" \
  DEVELOPMENT_TEAM="$DEVELOPMENT_TEAM" \
  CODE_SIGN_STYLE=Automatic \
  archive

cat > "$EXPORT_OPTIONS" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>method</key>
  <string>$EXPORT_METHOD</string>
  <key>signingStyle</key>
  <string>automatic</string>
  <key>teamID</key>
  <string>$DEVELOPMENT_TEAM</string>
  <key>stripSwiftSymbols</key>
  <true/>
  <key>compileBitcode</key>
  <false/>
</dict>
</plist>
PLIST

xcodebuild \
  -exportArchive \
  -archivePath "$ARCHIVE_PATH" \
  -exportPath "$EXPORT_DIR" \
  -exportOptionsPlist "$EXPORT_OPTIONS" \
  DEVELOPMENT_TEAM="$DEVELOPMENT_TEAM"

IPA_PATH="$(find "$EXPORT_DIR" -maxdepth 1 -name '*.ipa' -print -quit)"
if [[ -z "$IPA_PATH" ]]; then
  echo "Archive export finished, but no IPA was found in $EXPORT_DIR."
  exit 1
fi

echo "Built IPA:"
echo "  $IPA_PATH"
