# olcRTC Call

macOS/iOS-клиент для olcRTC.

Apple-часть отвечает за нативный интерфейс, хранение профилей, Keychain,
системный прокси/VPN-интеграцию и упаковку приложений. Go-runtime лежит в
`olcrtc/`.

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

## Сборка

### Сборка неподписанного iOS-клиента

Неподписанная сборка предназначена для local SOCKS режима на реальном iPhone
без Network Extension entitlement:

```bash
./apple/Scripts/build-ios-unsigned-local-ipa.sh
```

Результат:

```text
apple/.build/ios-unsigned-local/OlcRTCClient-unsigned-local.ipa
```

Этот IPA собирается с `LOCAL_SOCKS_ONLY`: в нем остается local SOCKS proxy, но
удаляется Packet Tunnel extension. Поэтому приложение не поднимает системный VPN
и не маршрутизирует весь iOS-трафик само. Системный трафик нужно направлять в
локальный прокси через стороннее приложение, например Happ, указав SOCKS5
`127.0.0.1:<port>`.

При старте local SOCKS на iOS в Events должно появиться:

```text
iOS background runtime is active for local SOCKS.
```

Установка через Sideloadly:

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
12. В Happ или другом приложении для маршрутизации трафика указать SOCKS5 proxy
    `127.0.0.1:<port>`.

Важно:

- Бесплатный Apple ID обычно требует периодической переустановки/обновления
  sideloaded app.
- Неподписанный local-SOCKS IPA не содержит Packet Tunnel extension.
- Если нужен системный VPN/Packet Tunnel без стороннего маршрутизатора, нужна
  подписанная сборка с правильными Apple Developer entitlements.

### Сборка macOS-клиента

Из корня репозитория:

```bash
./apple/Scripts/build-macos-app.sh
open ./apple/.build/olcRTC.app
```

Скрипт собирает:

- `apple/.build/olcrtc-macos`: Go CLI helper.
- `apple/.build/olcRTC.app`: запускаемый macOS app bundle.

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

### Сборка подписанного iOS-клиента

Подписанная сборка нужна для полной VPN/Packet Tunnel версии. Для нее нужен
Apple Developer Team и provisioning profiles с Network Extension
`packet-tunnel-provider` entitlement для обоих iOS targets:

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

## Дополнительно

- Детали структуры проекта, XcodeGen и ограничения: [docs/dev.md](docs/dev.md).
- Формат профилей и подписок описан в `olcrtc/docs/sub.md` и
  `olcrtc/docs/uri.md`.
