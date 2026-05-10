# olcRTC

olcRTC builds encrypted client/server tunnels over WebRTC-based carriers. The
core runtime is written in Go. Desktop and Apple mobile UI live in `apple/`.

## Current Status

The project is beta software.

- Go CLI supports server/client modes, profile URI/subscription formats, and
  local SOCKS5 client access.
- macOS has a SwiftUI app that bundles the Go CLI helper, starts the local
  SOCKS5 endpoint, and can enable the selected macOS system SOCKS proxy.
- iOS has a SwiftUI app plus a `NetworkExtension` packet tunnel target. The
  packet tunnel starts the gomobile olcRTC runtime and bridges device traffic to
  the local SOCKS5 endpoint with `Tun2SocksKit` / `hev-socks5-tunnel`.
- iOS simulator builds work for development when the app is signed normally
  with simulated entitlements. Real iPhone builds require Apple Developer
  provisioning profiles with the Network Extension `packet-tunnel-provider`
  entitlement for both the app and the packet tunnel extension.
- The iOS packet tunnel currently focuses on TCP and DNS-over-tunnel behavior.
  Arbitrary UDP is not yet a complete production path.

## Repository Layout

- `cmd/olcrtc`: Go CLI entry point.
- `internal`: tunnel runtime, carriers, transports, crypto, SOCKS client/server,
  and tests.
- `mobile`: gomobile-compatible API used by Apple/iOS clients.
- `docs`: protocol, settings, URI, subscription, and manual usage docs.
- `apple`: native macOS/iOS SwiftUI clients, XcodeGen project spec, scripts, and
  iOS packet tunnel extension.
- `script`: helper scripts for CLI/server/container workflows.
- `data`: default name data used by the runtime.

Generated artifacts are intentionally not tracked: `apple/.build`,
`apple/.derived-data`, `apple/.swiftpm`, and
`apple/Frameworks/Mobile.xcframework`.

## General CLI Build

Install Mage once:

```bash
go install github.com/magefile/mage@latest
```

Common commands:

```bash
mage build      # build CLI + UI artifacts used by the Go workflow
mage buildCLI   # build CLI only
mage cross      # cross-compile for Linux, Windows, and Darwin
mage mobile     # build Android AAR through gomobile
mage test       # run tests
mage lint       # run linters
mage clean      # remove generated build outputs
```

Container image helpers:

```bash
mage podman
mage docker
```

## Build the macOS App

Requirements:

- macOS 13 or newer
- Go toolchain
- Xcode or Command Line Tools with SwiftPM

Build and launch:

```bash
./apple/Scripts/build-macos-app.sh
open ./apple/.build/olcRTC.app
```

The script builds:

- `apple/.build/olcrtc-macos`: bundled Go CLI helper
- `apple/.build/olcRTC.app`: SwiftUI macOS app bundle

The macOS app stores profile secrets in Keychain. Non-secret profile metadata is
stored in `UserDefaults`. When the tunnel starts successfully, the Events pane
should show the local SOCKS address and, if enabled, the selected macOS network
service proxy state.

If the app is force-quit while the system proxy is enabled, turn it off manually
if needed:

```bash
networksetup -setsocksfirewallproxystate "Wi-Fi" off
```

## Test the iOS App in Simulator

Requirements:

- Full Xcode, not only Command Line Tools
- An installed iOS simulator runtime
- Go toolchain
- `gomobile`; the script installs it if missing
- `xcodegen` is recommended; the script regenerates the Xcode project when it
  is installed

Run:

```bash
./apple/Scripts/run-ios-simulator.sh
```

The script:

1. Ensures Xcode and the iOS SDK are usable.
2. Runs `gomobile init`.
3. Builds `apple/Frameworks/Mobile.xcframework`.
4. Regenerates `apple/OlcRTCClient.xcodeproj` when `xcodegen` is available.
5. Builds the `OlcRTCClient iOS` scheme with normal local simulator signing.
6. Installs and launches the app on a booted or newly booted simulator.

Do not build the simulator app with `CODE_SIGNING_ALLOWED=NO` when testing the
VPN path. The packet tunnel extension needs simulated Network Extension
entitlements, otherwise `startVPNTunnel` can fail with `IPC failed`.

To build without launching:

```bash
./apple/Scripts/build-ios-app.sh
```

## Test the iOS App on a Real iPhone

Open the Xcode project:

```bash
open ./apple/OlcRTCClient.xcodeproj
```

In Xcode:

1. Select target `OlcRTCClient iOS`.
2. Set your Apple Developer Team.
3. Enable automatic signing or choose a provisioning profile for the app bundle.
4. Select target `OlcRTCPacketTunnel`.
5. Set the same Team and choose a provisioning profile for the extension bundle.
6. Ensure both profiles include the Network Extension capability with
   `packet-tunnel-provider`.

The two bundle identifiers are:

```text
community.openlibre.olcrtc.ios
community.openlibre.olcrtc.ios.PacketTunnel
```

Both must be explicit App IDs in Apple Developer Certificates, Identifiers &
Profiles. Wildcard App IDs will not work for Network Extension.

## Apple Project Notes

The canonical Apple project description is `apple/project.yml`. Regenerate the
Xcode project after changing targets, dependencies, signing settings, or bundle
identifiers:

```bash
cd apple
xcodegen generate
```

The checked-in Xcode project is kept for convenience, but `project.yml` should
remain the source of truth for structural changes.

## Documentation

- [Fast start](docs/fast.md)
- [Manual](docs/manual.md)
- [Settings matrix](docs/settings.md)
- [Client URI format](docs/uri.md)
- [Client subscription format](docs/sub.md)
- [Apple client notes](apple/README.md)

## Contacts

- Telegram: [@openlibrecommunity](https://t.me/openlibrecommunity)
- Community Android client: [alananisimov/olcbox](https://github.com/alananisimov/olcbox)
