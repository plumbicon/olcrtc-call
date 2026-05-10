# olcRTC Apple Client

Native SwiftUI clients for macOS and iOS.

The Go runtime remains the source of truth. The Apple layer owns native UI,
profile storage, Keychain secrets, platform proxy/VPN integration, and packaging.

## Current Shape

- macOS app: launches a bundled `olcrtc` CLI helper, waits for the local SOCKS5
  endpoint, and can enable the selected macOS system SOCKS proxy.
- iOS app: links `Mobile.xcframework`, stores/imports profiles, and starts an
  iOS `NetworkExtension` packet tunnel.
- iOS packet tunnel: starts gomobile olcRTC inside the extension and runs
  `Tun2SocksKit` / `hev-socks5-tunnel` to bridge packet tunnel traffic to the
  local SOCKS5 endpoint.
- Subscriptions: imports `olcrtc://` URIs, subscription URLs, or pasted
  `sub.md` content. HTTP(S) subscription fetches retry through DNS-over-HTTPS
  when normal DNS lookup fails.

## Layout

- `Package.swift`: SwiftPM package for shared kit and the macOS development
  executable.
- `project.yml`: XcodeGen source of truth for app/extension targets.
- `OlcRTCClient.xcodeproj`: generated Xcode project checked in for convenience.
- `Frameworks/.gitkeep`: placeholder for generated gomobile frameworks.
- `Sources/OlcRTCClientKit`: shared SwiftUI views, models, stores, parsers, and
  runtime managers.
- `Sources/OlcRTCClientMac`: macOS app entry point.
- `Sources/OlcRTCClientiOS`: iOS app entry point and app entitlements.
- `Sources/OlcRTCPacketTunnel`: iOS packet tunnel provider, Info.plist, and
  extension entitlements.
- `Scripts`: reproducible build/test helpers.

Generated local outputs:

- `.build/`
- `.derived-data/`
- `.swiftpm/`
- `Frameworks/Mobile.xcframework`

These outputs are ignored by git and can be deleted at any time.

## Prerequisites

For macOS app builds:

- macOS 13 or newer
- Go
- Xcode or Command Line Tools with SwiftPM

For iOS builds:

- Full Xcode with iOS SDK and simulator runtime
- Go
- `gomobile`
- `xcodegen` for regenerating `OlcRTCClient.xcodeproj`

If Xcode was just installed:

```bash
sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
sudo xcodebuild -license accept
```

## Build the macOS App

From the repository root:

```bash
./apple/Scripts/build-macos-app.sh
open ./apple/.build/olcRTC.app
```

The script builds:

- `.build/olcrtc-macos`: Go CLI helper
- `.build/olcRTC.app`: launchable macOS app bundle

The macOS app stores profile secrets in Keychain. The JSON profile stored in
`UserDefaults` does not contain the encryption key or SOCKS password.

When Start succeeds, Events should show:

```text
SOCKS proxy is ready at 127.0.0.1:<port>.
System SOCKS proxy enabled for <service> on 127.0.0.1:<port>.
```

If system proxy is disabled in the UI, only apps manually configured to use the
local SOCKS endpoint will use olcRTC.

If the app is force-quit while macOS system proxy is enabled:

```bash
networksetup -setsocksfirewallproxystate "Wi-Fi" off
```

## Build the Go Apple Framework

From the repository root:

```bash
./apple/Scripts/build-xcframework.sh
```

This creates:

```text
apple/Frameworks/Mobile.xcframework
```

The framework is generated from `./mobile` using `gomobile bind` for iOS,
iOS Simulator, and macOS.

## Test the iOS App in Simulator

From the repository root:

```bash
./apple/Scripts/run-ios-simulator.sh
```

The script builds `Mobile.xcframework`, regenerates the Xcode project when
`xcodegen` is installed, builds the `OlcRTCClient iOS` scheme, installs the app
on a simulator, and launches it.

The simulator build must be signed normally. Do not add
`CODE_SIGNING_ALLOWED=NO` while testing the packet tunnel; without simulated
Network Extension entitlements, `startVPNTunnel` can fail with `IPC failed`.

Build without launching:

```bash
./apple/Scripts/build-ios-app.sh
```

## Test on a Real iPhone

Open the project:

```bash
open ./apple/OlcRTCClient.xcodeproj
```

Set signing for both targets:

- `OlcRTCClient iOS`
- `OlcRTCPacketTunnel`

Both targets need provisioning profiles with:

```xml
<key>com.apple.developer.networking.networkextension</key>
<array>
    <string>packet-tunnel-provider</string>
</array>
```

Use explicit App IDs:

```text
community.openlibre.olcrtc.ios
community.openlibre.olcrtc.ios.PacketTunnel
```

If you change bundle IDs, keep the extension bundle ID prefixed by the app
bundle ID and update both Xcode targets.

## Regenerate the Xcode Project

After structural changes to targets, dependencies, entitlements, or bundle IDs:

```bash
cd apple
xcodegen generate
```

`project.yml` should be treated as the source of truth. The checked-in
`OlcRTCClient.xcodeproj` is generated from it for convenience.

## Profiles and Subscriptions

Use the add button to create a profile manually.

Use the import button to add:

- a single `olcrtc://` profile URI
- an HTTP/HTTPS subscription URL
- pasted subscription text in the documented `sub.md` format

Refreshing a subscription updates matching nodes, adds new nodes, and removes
nodes missing from the refreshed source. Local runtime settings such as SOCKS
port, SOCKS credentials, DNS, debug logging, and timeout are preserved when a
node can be matched across refreshes.

## Known Limits

- iOS packet tunnel currently focuses on TCP and DNS-over-tunnel behavior.
  Arbitrary UDP forwarding is not yet a complete production path.
- Real iPhone testing requires a paid Apple Developer Program account and
  Network Extension capable provisioning profiles for both targets.
- `Mobile.xcframework` is generated and intentionally not committed.
