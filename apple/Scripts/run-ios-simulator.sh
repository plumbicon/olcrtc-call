#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APPLE_DIR="$ROOT_DIR/apple"
SCHEME="OlcRTCClient iOS"
BUNDLE_ID="community.openlibre.olcrtc.ios"
DERIVED_DATA="$APPLE_DIR/.derived-data/ios-simulator"

if [[ -z "${DEVELOPER_DIR:-}" && -d /Applications/Xcode.app/Contents/Developer ]]; then
  export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
fi

if ! xcodebuild -version >/dev/null 2>&1; then
  cat <<'MSG'
Xcode is installed but not ready for iOS Simulator.

Run these once in Terminal:
  sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
  sudo xcodebuild -license accept

Then rerun:
  ./apple/Scripts/run-ios-simulator.sh
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
  ./apple/Scripts/run-ios-simulator.sh
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

DEVICE_ID="$(xcrun simctl list devices available | \
  sed -n 's/.*(\([0-9A-Fa-f-]\{36\}\)) (Booted).*/\1/p' | head -1)"

if [[ -z "$DEVICE_ID" ]]; then
  DEVICE_ID="$(xcrun simctl list devices available | \
    sed -n 's/.*(\([0-9A-Fa-f-]\{36\}\)) (Shutdown).*/\1/p' | head -1)"
  if [[ -z "$DEVICE_ID" ]]; then
    RUNTIME_ID="$(xcrun simctl list runtimes available | \
      awk -F'[()]' '/iOS/ && /com.apple.CoreSimulator.SimRuntime/ { value=$2 } END { print value }')"
    DEVICE_TYPE_ID="$(xcrun simctl list devicetypes available | \
      awk -F'[()]' '/iPhone/ && /com.apple.CoreSimulator.SimDeviceType/ { value=$2 } END { print value }')"

    if [[ -z "$RUNTIME_ID" || -z "$DEVICE_TYPE_ID" ]]; then
      echo "No available iOS Simulator runtime/device type found. Install an iOS simulator runtime in Xcode."
      exit 1
    fi

    DEVICE_ID="$(xcrun simctl create "olcRTC iPhone" "$DEVICE_TYPE_ID" "$RUNTIME_ID")"
  fi
  xcrun simctl boot "$DEVICE_ID" || true
fi

open -a Simulator

xcodebuild \
  -project "$APPLE_DIR/OlcRTCClient.xcodeproj" \
  -scheme "$SCHEME" \
  -configuration Debug \
  -destination "id=$DEVICE_ID" \
  -derivedDataPath "$DERIVED_DATA" \
  build

APP_PATH="$(find "$DERIVED_DATA/Build/Products/Debug-iphonesimulator" -maxdepth 1 -name '*.app' -print -quit)"
if [[ -z "$APP_PATH" ]]; then
  echo "Built app bundle was not found in $DERIVED_DATA."
  exit 1
fi

xcrun simctl install "$DEVICE_ID" "$APP_PATH"
xcrun simctl launch "$DEVICE_ID" "$BUNDLE_ID"

echo "Launched olcRTC on iOS Simulator device $DEVICE_ID"
