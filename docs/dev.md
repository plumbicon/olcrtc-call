# Разработка

## Структура

- `olcrtc/`: Go-runtime, CLI, тесты, data-файлы и gomobile-пакет.
- `apple/Package.swift`: SwiftPM-пакет с общим кодом приложения.
- `apple/project.yml`: основной файл XcodeGen для app/extension targets.
- `apple/Godwit.xcodeproj`: сгенерированный Xcode-проект.
- `apple/Sources/OlcRTCClientKit`: общие SwiftUI views, models, stores,
  parsers и runtime managers.
- `apple/Sources/OlcRTCClientMac`: точка входа macOS-приложения.
- `apple/Sources/OlcRTCClientiOS`: точка входа iOS-приложения и entitlements.
- `apple/Sources/OlcRTCPacketTunnel`: iOS Packet Tunnel extension.
- `apple/Scripts`: скрипты сборки.

Локальные результаты сборки не коммитятся:

- `apple/.build/`
- `apple/.derived-data/`
- `apple/.swiftpm/`
- `apple/Frameworks/Mobile.xcframework`

## Проект Xcode

После изменений targets, dependencies, entitlements или bundle IDs:

```bash
cd apple
xcodegen generate
```

`project.yml` считается единственным источником правды. Xcode-проект
генерируется из него.

Для быстрой проверки доступных targets и schemes:

```bash
xcodebuild -list -project apple/Godwit.xcodeproj
```

## Ограничения

- iOS Packet Tunnel сейчас сфокусирован на TCP и DNS-over-tunnel поведении.
  Произвольный UDP forwarding еще не является полноценным production path.
- iOS local SOCKS mode использует background audio mode, чтобы процесс
  продолжал работать после сворачивания приложения. Это удобно для sideloaded
  local testing; для системного iOS-трафика нужен сторонний маршрутизатор
  трафика или подписанная Packet Tunnel сборка.
- Для реального iPhone с Packet Tunnel нужен платный Apple Developer Program и
  provisioning profiles с Network Extension capability для обоих iOS targets.
