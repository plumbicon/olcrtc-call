# Apple Client TODO

This is the implementation plan for a native iOS/macOS wrapper around the
existing `mobile` Go package. The Go runtime remains the source of truth; the
Apple layer owns only UI, local profile storage, and platform integration.

## Phase 0 - Repository Setup

- [x] Keep Apple-specific code under `apple/`.
- [x] Add a SwiftUI client shell shared by iOS and macOS.
- [x] Add a script that builds the Go package as an Apple XCFramework.
- [x] Generate an Xcode project; bundle/team IDs can be changed in Xcode.
- [x] Add a macOS CLI fallback engine for development before the gomobile
      Apple framework is available.

## Phase 1 - Local SOCKS MVP

- [x] Profile editor for carrier, transport, room, client ID, key, SOCKS port,
      SOCKS credentials, DNS, debug mode, and VP8 options.
- [x] Start/stop controls that call the gomobile API when the framework is
      linked.
- [x] Readiness wait with clear UI states.
- [x] Local logs/events pane for user-visible lifecycle messages.
- [x] Persist profile secrets in Keychain instead of plain profile JSON.
- [x] Add URI import for `olcrtc://...` links.
- [x] Add subscription import and manual refresh from source URL.
- [x] Add validation for 64-char hex keys and carrier-specific room rules.
- [x] Add macOS system SOCKS proxy control for selected network services.
- [x] Add launchable macOS `.app` bundle packaging with CLI helper and data.
- [ ] Add a smoke test profile flow on macOS.

## Phase 2 - iPhone Device Testing

- [ ] Set the app bundle ID and Apple team ID.
- [x] Add scripts that build `Mobile.xcframework` locally with full Xcode +
      gomobile.
- [x] Link the framework into the iOS target when generated.
- [ ] Test launch, start, wait-ready, stop, background/foreground transitions.
- [x] Keep manual local SOCKS mode available for debugging on iOS.

## Phase 3 - System Tunnel

- [x] Decide whether the production iOS client must route traffic for other
      apps.
- [x] If yes, add a `NetworkExtension` packet tunnel target.
- [ ] Add entitlement request/review notes for Apple.
- [x] Design the bridge between packet tunnel traffic and olcRTC/SOCKS
      (`tun2socks` or equivalent).
- [ ] Add socket protection/route exclusion equivalent for tunnel-owned sockets
      where needed.
- [x] Document simulator signing requirements for the packet tunnel.

## Phase 4 - App Store/TestFlight Readiness

- [ ] Prepare privacy labels and a short beta review note explaining the tunnel.
- [ ] Add App Transport Security/network usage descriptions where required.
- [ ] Add basic telemetry-free diagnostics export for bug reports.
- [ ] Harden profile storage, logging redaction, and crash-safe stop behavior.

## Phase 5 - Android Reuse

- [ ] Keep Android out of the shared UI unless there is a strong reason to unify
      the product.
- [ ] Reuse only cross-platform domain logic that does not dilute the native
      iOS/macOS experience.
