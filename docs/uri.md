<div align="center">

<img src="https://github.com/openlibrecommunity/material/blob/master/olcrtc.png" width="250" height="250">

![License](https://img.shields.io/badge/license-WTFPL-0D1117?style=flat-square&logo=open-source-initiative&logoColor=green&labelColor=0D1117)
![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)

</div>


# Краткий URI-формат для клиентов

Этот документ описывает **соглашение для разработчиков клиентских приложений**, которым нужен компактный способ передавать параметры подключения `olcrtc`.

Текущий `olcrtc` не парсит такой URI автоматически. Если клиентское приложение хочет использовать эту запись, оно должно само разобрать строку и передать полученные поля в свои вызовы `olcrtc`.

---

## Формат

```text
olcrtc://<Carrier>?<Transport>@<RoomID>#<EncryptionKey>%<ClientID>$<MIMO>
olcrtc://<Carrier>?<Transport><key=value&key=value>@<RoomID>#<EncryptionKey>%<ClientID>$<MIMO>
```

Все поля после `olcrtc://` считаются частью клиентского соглашения.

Блок `<key=value&...>` — payload параметров транспорта в угловых скобках, идёт сразу после имени транспорта. Если параметры транспорту не нужны или используются defaults — блок опускается целиком.

---

## Поля

| Поле | Значение |
|------|----------|
| `<Carrier>` | Имя carrier, например `telemost`, `jazz`, `wbstream` |
| `<Transport>` | Имя транспорта, например `datachannel`, `vp8channel`, `seichannel`, `videochannel` |
| payload | Параметры транспорта в `<key=value&...>`. Ключи совпадают с CLI-флагами без дефиса. Блок опускается если используются defaults |
| `<RoomID>` | Идентификатор комнаты или carrier-specific room URL/ID |
| `<EncryptionKey>` | Ключ шифрования в hex, обычно 64 символа (`32` байта) |
| `<ClientID>` | Идентификатор клиента. Должен совпадать с ожидаемым значением на сервере |
| `<MIMO>` | Свободный комментарий для UI/метаданных, например `RU / olc free sub / IPv6` |

---

## Параметры payload по транспортам

### datachannel

Payload не используется.

### vp8channel

| Ключ | CLI-флаг | Описание |
|------|----------|----------|
| `vp8-fps` | `-vp8-fps` | FPS VP8 потока |
| `vp8-batch` | `-vp8-batch` | Кадров за тик |

### seichannel

| Ключ | CLI-флаг | Описание |
|------|----------|----------|
| `fps` | `-fps` | FPS H264 потока |
| `batch` | `-batch` | Кадров за тик |
| `frag` | `-frag` | Размер фрагмента в байтах |
| `ack-ms` | `-ack-ms` | Таймаут ACK в миллисекундах |

### videochannel

| Ключ | CLI-флаг | Описание |
|------|----------|----------|
| `video-w` | `-video-w` | Ширина в пикселях |
| `video-h` | `-video-h` | Высота в пикселях |
| `video-fps` | `-video-fps` | FPS |
| `video-bitrate` | `-video-bitrate` | Битрейт, например `5000k` или `2M` |
| `video-hw` | `-video-hw` | Аппаратное ускорение: `none` или `nvenc` |
| `video-codec` | `-video-codec` | `qrcode` или `tile` |
| `video-qr-size` | `-video-qr-size` | Размер фрагмента QR в байтах |
| `video-qr-recovery` | `-video-qr-recovery` | Коррекция ошибок: `low` / `medium` / `high` / `highest` |
| `video-tile-module` | `-video-tile-module` | Размер тайла в пикселях 1..270 (только `tile`) |
| `video-tile-rs` | `-video-tile-rs` | Reed-Solomon паритет % 0..200 (только `tile`) |

---

## Соответствие параметрам olcrtc

| URI поле | Параметр / значение |
|----------|---------------------|
| `<Carrier>` | `-carrier` |
| `<Transport>` | `-transport` |
| payload | соответствующие флаги транспорта |
| `<RoomID>` | `-id` |
| `<EncryptionKey>` | `-key` |
| `<ClientID>` | `-client-id` |
| `<MIMO>` | В `olcrtc` не передаётся. Это только клиентский комментарий |

`-link direct` и `-data data` в этом формате не кодируются, потому что для текущих сценариев они фиксированные.

---

## Разделители

| Разделитель | После него идёт |
|-------------|-----------------|
| `://` | начало полезной нагрузки после схемы `olcrtc` |
| `?` | `<Transport>` |
| `<...>` | payload параметров транспорта |
| `@` | `<RoomID>` |
| `#` | `<EncryptionKey>` |
| `%` | `<ClientID>` |
| `$` | `<MIMO>` |

Рекомендуется не использовать эти символы внутри самих полей. Если клиенту это нужно, он должен ввести собственное escaping/percent-encoding правило и применять его симметрично при кодировании и декодировании.

---

## Примеры

### wbstream + datachannel (рекомендуется)

```text
olcrtc://wbstream?datachannel@room-01#d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799%android-01$RU / olc free sub / IPv6
```

Payload не нужен — datachannel параметров не имеет.

### Эквивалент CLI

```sh
./olcrtc -mode cnc \
  -carrier wbstream \
  -transport datachannel \
  -id room-01 \
  -client-id android-01 \
  -key d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799 \
  -link direct \
  -data data
```

### wbstream + vp8channel

```text
olcrtc://wbstream?vp8channel<vp8-fps=60&vp8-batch=64>@room-01#d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799%android-01$RU / olc free sub / IPv6
```

### Эквивалент CLI

```sh
./olcrtc -mode cnc \
  -carrier wbstream \
  -transport vp8channel \
  -id room-01 \
  -client-id android-01 \
  -key d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799 \
  -link direct \
  -data data \
  -vp8-fps 60 -vp8-batch 64
```

### jazz + seichannel

```text
olcrtc://jazz?seichannel<fps=60&batch=64&frag=900&ack-ms=2000>@room-01#d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799%android-01$DE / olc free sub
```

### Эквивалент CLI

```sh
./olcrtc -mode cnc \
  -carrier jazz \
  -transport seichannel \
  -id room-01 \
  -client-id android-01 \
  -key d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799 \
  -link direct \
  -data data \
  -fps 60 -batch 64 -frag 900 -ack-ms 2000
```

### telemost + videochannel

```text
olcrtc://telemost?videochannel<video-w=1080&video-h=1080&video-fps=60&video-bitrate=5000k&video-hw=none&video-codec=qrcode>@room-01#d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799%android-01$MIMO
```

### Эквивалент CLI

```sh
./olcrtc -mode cnc \
  -carrier telemost \
  -transport videochannel \
  -id room-01 \
  -client-id android-01 \
  -key d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799 \
  -link direct \
  -data data \
  -video-w 1080 -video-h 1080 -video-fps 60 -video-bitrate 5000k -video-hw none -video-codec qrcode
```

---

## Короткие алиасы

Как хотите но лично я был бы против.

---

Формат подписки (список серверов): [sub.md](sub.md)

Матрица совместимости carrier + transport: [settings.md](settings.md)
