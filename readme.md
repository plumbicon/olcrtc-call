# olcRTC Call

macOS/iOS-клиент для olcRTC, собранный вокруг Go-runtime.

Go-runtime остается источником сетевой логики. Apple-часть отвечает за нативный
интерфейс, хранение профилей, Keychain, системный прокси/VPN-интеграцию и
упаковку приложений.

## Структура

- `olcrtc/`: Go-runtime, CLI, тесты, data-файлы и gomobile-пакет.
- `apple/Package.swift`: SwiftPM-пакет с общим кодом приложения.
- `apple/project.yml`: основной файл XcodeGen для app/extension targets.
- `apple/OlcRTCClient.xcodeproj`: сгенерированный Xcode-проект.
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

## Требования

- macOS 13 или новее.
- Go.
- Xcode или Command Line Tools для macOS/SwiftPM-сборок.
- Полный Xcode с iOS SDK для iOS-сборок.
- `gomobile`.
- `xcodegen`, если нужно пересоздавать `apple/OlcRTCClient.xcodeproj`.
- Sideloadly, если нужно установить неподписанный local-SOCKS IPA на iPhone.

Если Xcode только что установлен:

```bash
sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
sudo xcodebuild -license accept
```

## Go-runtime

```bash
cd olcrtc
go test ./...
```

## Сборка macOS-клиента

Из корня репозитория:

```bash
./apple/Scripts/build-macos-app.sh
open ./apple/.build/olcRTC.app
```

Скрипт собирает:

- `apple/.build/olcrtc-macos`: Go CLI helper.
- `apple/.build/olcRTC.app`: запускаемый macOS app bundle.

macOS-приложение хранит секреты профиля в Keychain. JSON-профиль в
`UserDefaults` не содержит ключ шифрования или SOCKS-пароль.

При успешном старте в Events должно появиться:

```text
SOCKS proxy is ready at 127.0.0.1:<port>.
System SOCKS proxy enabled for <service> on 127.0.0.1:<port>.
```

Если приложение было принудительно закрыто, пока системный SOCKS-прокси macOS
включен:

```bash
networksetup -setsocksfirewallproxystate "Wi-Fi" off
```

## Общая iOS-сборка

Собрать gomobile framework:

```bash
./apple/Scripts/build-xcframework.sh
```

Запустить iOS-приложение в Simulator:

```bash
./apple/Scripts/run-ios-simulator.sh
```

Только собрать iOS-приложение для Simulator, без запуска:

```bash
./apple/Scripts/build-ios-app.sh
```

## Неподписанный iOS-клиент

Для установки local-SOCKS версии на реальный iPhone без Network Extension
entitlement:

```bash
./apple/Scripts/build-ios-unsigned-local-ipa.sh
```

Результат:

```text
apple/.build/ios-unsigned-local/OlcRTCClient-unsigned-local.ipa
```

Этот IPA собирается с `LOCAL_SOCKS_ONLY`: в нем остается local SOCKS режим, но
удаляется Packet Tunnel extension. Это нужно, чтобы пакет можно было затем
подписать обычным Apple ID через Sideloadly без Network Extension entitlement.

При старте local SOCKS на iOS в Events должно появиться:

```text
iOS background runtime is active for local SOCKS.
```

## Установка неподписанного IPA через Sideloadly

1. Установить Sideloadly с официального сайта: `https://sideloadly.io/`.
2. Подключить iPhone к Mac по USB и нажать Trust/Доверять на телефоне, если iOS
   спросит.
3. Собрать IPA:

   ```bash
   ./apple/Scripts/build-ios-unsigned-local-ipa.sh
   ```

4. Открыть Sideloadly.
5. Перетащить файл
   `apple/.build/ios-unsigned-local/OlcRTCClient-unsigned-local.ipa` в окно
   Sideloadly или выбрать его через кнопку IPA.
6. Выбрать подключенный iPhone в списке устройств.
7. Ввести Apple ID. Для local-SOCKS IPA подходит обычный бесплатный Apple ID.
   Лучше использовать отдельный Apple ID для sideloading, а не основной.
8. Нажать Start и дождаться завершения установки.
9. На iPhone открыть Settings -> General -> VPN & Device Management и доверить
   developer profile, связанный с использованным Apple ID.
10. На iOS 16 и новее включить Developer Mode: Settings -> Privacy & Security
    -> Developer Mode, затем перезагрузить устройство, если iOS попросит.
11. Запустить olcRTC на iPhone, выбрать профиль и нажать Start.
12. В приложениях, которые должны идти через olcRTC, вручную указать SOCKS5
    proxy `127.0.0.1:<port>`.

Важно:

- Бесплатный Apple ID обычно требует периодической переустановки/обновления
  sideloaded app.
- Неподписанный local-SOCKS IPA не содержит Packet Tunnel extension и не
  включает системный VPN для всего iOS-трафика.
- Если нужен системный VPN/Packet Tunnel, нужен подписанный IPA с правильными
  Apple Developer entitlements.

## Подписанный iOS-клиент

Для полной VPN/Packet Tunnel версии нужен Apple Developer Team и provisioning
profiles с Network Extension `packet-tunnel-provider` entitlement для обоих
iOS targets:

- `OlcRTCClient iOS`
- `OlcRTCPacketTunnel`

IPA для разработки:

```bash
DEVELOPMENT_TEAM=ABCDE12345 \
EXPORT_METHOD=development \
./apple/Scripts/build-ios-ipa.sh
```

Ad-hoc IPA:

```bash
DEVELOPMENT_TEAM=ABCDE12345 \
EXPORT_METHOD=ad-hoc \
./apple/Scripts/build-ios-ipa.sh
```

Поддерживаемые значения `EXPORT_METHOD`:

- `development`
- `ad-hoc`
- `app-store`
- `enterprise`

Скрипт пишет архив и экспортированный IPA сюда:

```text
apple/.build/ios-archive/
apple/.build/ios-ipa/
```

Для тестирования с реального iPhone через Xcode:

```bash
open ./apple/OlcRTCClient.xcodeproj
```

В Xcode нужно настроить signing для обоих targets:

- `OlcRTCClient iOS`
- `OlcRTCPacketTunnel`

Используемые bundle IDs:

```text
community.openlibre.olcrtc.ios
community.openlibre.olcrtc.ios.PacketTunnel
```

Если bundle IDs меняются, extension bundle ID должен начинаться с bundle ID
основного приложения.

## Проект Xcode

После изменений targets, dependencies, entitlements или bundle IDs:

```bash
cd apple
xcodegen generate
```

`project.yml` считается единственным источником правды. Xcode-проект
генерируется из него.

## Профили и подписки

Через import можно добавить:

- одиночный `olcrtc://` profile URI;
- HTTP/HTTPS subscription URL;
- вставленный текст подписки в формате `sub.md`.

При refresh подписки приложение обновляет найденные nodes, добавляет новые и
удаляет отсутствующие в обновленном источнике. Локальные runtime-настройки вроде
SOCKS-порта, SOCKS credentials, DNS, debug logging и timeout сохраняются, если
node можно сопоставить между refresh.

## Ограничения

- iOS Packet Tunnel сейчас сфокусирован на TCP и DNS-over-tunnel поведении.
  Произвольный UDP forwarding еще не является полноценным production path.
- iOS local SOCKS mode использует background audio mode, чтобы процесс
  продолжал работать после сворачивания приложения. Это удобно для sideloaded
  local testing; для системного iOS-трафика чище использовать Packet Tunnel.
- Для реального iPhone с Packet Tunnel нужен платный Apple Developer Program и
  provisioning profiles с Network Extension capability для обоих iOS targets.
