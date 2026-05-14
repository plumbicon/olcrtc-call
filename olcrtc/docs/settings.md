<div align="center">

<img src="https://github.com/openlibrecommunity/material/blob/master/olcrtc.png" width="250" height="250">

![License](https://img.shields.io/badge/license-WTFPL-0D1117?style=flat-square&logo=open-source-initiative&logoColor=green&labelColor=0D1117)
![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)

</div>


# Настройки

## Матрица совместимости

| Transport | telemost | jazz | wbstream |
|-----------|:--------:|:----:|:--------:|
| datachannel | - | * | + |
| vp8channel | + | + | + |
| seichannel | - | + | + |
| videochannel | + | + | + |

**Легенда:**
- `+` - работает
- `-` - не поддерживается
- `*` - работает, но не желательно

**Рекомендуемая комбинация: `wbstream + datachannel`** - максимальная скорость, минимальный пинг.

Скорость по убыванию: `datachannel` > `vp8channel` > `seichannel` > `videochannel`

---

## Обязательные флаги

| Флаг | Что вводить |
|------|-------------|
| `-mode` | `srv` на сервере, `cnc` на клиенте, `gen` для генерации Room ID |
| `-carrier` | `telemost`, `jazz` или `wbstream` |
| `-transport` | `datachannel`, `vp8channel`, `seichannel` или `videochannel` |
| `-id` | Room ID |
| `-client-id` | Общий идентификатор клиента. Должен совпадать на сервере и клиенте. Один client-id может держать бесконечное количество соединений, но SFU ограничивает полосу на участника - оптимально 1 client-id = 1 пользователь (не обязательно) |
| `-key` | Ключ шифрования hex 64 символа. Генерация: `openssl rand -hex 32` |
| `-link` | Всегда `direct` |
| `-data` | Всегда `data` |
| `-dns` | DNS-сервер, например `1.1.1.1:53` |

---

## Необязательные флаги

| Флаг | Описание |
|------|----------|
| `--debug` | Подробные логи соединений |

---

## -mode gen

Генерирует Room ID заранее, не запуская сервер. Поддерживается только для `jazz`. Для `wbstream` создавай руму вручную через [stream.wb.ru](https://stream.wb.ru) (автогенерация отключена со стороны WB).

**Обязательные флаги:**

| Флаг | Описание |
|------|----------|
| `-carrier` | `jazz` |
| `-dns` | DNS-сервер |
| `-amount` | Количество комнат |

```sh
./olcrtc -mode gen -carrier jazz -dns 1.1.1.1:53 -amount 3
# room-id-1
# room-id-2
# room-id-3
```

---

## Флаги только для сервера (`-mode srv`)

| Флаг | Описание |
|------|----------|
| `-socks-proxy` | Адрес SOCKS5-прокси для исходящего трафика сервера |
| `-socks-proxy-port` | Порт этого прокси |

---

## Флаги только для клиента (`-mode cnc`)

| Флаг | Описание | По умолчанию |
|------|----------|:------------:|
| `-socks-host` | На каком адресе поднять SOCKS5 | `127.0.0.1` |
| `-socks-port` | На каком порту поднять SOCKS5 | `1080` |
| `-socks-user` | Логин для входящих SOCKS5-подключений (необязательно) | - |
| `-socks-pass` | Пароль для входящих SOCKS5-подключений (необязательно) | - |

Если `-socks-user` не задан - аутентификация отключена (любой локальный клиент может подключиться).  
Если задан - клиент принимает только подключения с правильным логином и паролем (RFC 1929).

---

## datachannel

Дополнительных флагов нет - всё по умолчанию.

---

## vp8channel

**Рекомендуется: `-vp8-fps 60 -vp8-batch 64`** (числа лучше чётные, больший batch = выше скорость)

| Флаг | Описание | По умолчанию |
|------|----------|:------------:|
| `-vp8-fps` | FPS VP8 потока | `25` |
| `-vp8-batch` | Кадров за тик | `1` |

---

## seichannel

**Рекомендуется: `-fps 60 -batch 64 -frag 900 -ack-ms 2000`**

| Флаг | Описание | По умолчанию |
|------|----------|:------------:|
| `-fps` | FPS H264 потока | `60` |
| `-batch` | Кадров за тик | `64` |
| `-frag` | Размер фрагмента в байтах | `900` |
| `-ack-ms` | Таймаут ACK в миллисекундах | `2000` |

---

## videochannel

**Рекомендуется: `-video-codec qrcode -video-w 1080 -video-h 1080 -video-fps 60 -video-bitrate 5000k -video-hw none`**

| Флаг | Описание | По умолчанию |
|------|----------|:------------:|
| `-video-codec` | `qrcode` или `tile` | `qrcode` |
| `-video-w` | Ширина в пикселях | `1920` |
| `-video-h` | Высота в пикселях | `1080` |
| `-video-fps` | FPS | `30` |
| `-video-bitrate` | Битрейт, например `2M` или `5000k` | `2M` |
| `-video-hw` | Аппаратное ускорение: `none` или `nvenc` | `none` |
| `-video-qr-recovery` | Коррекция ошибок QR: `low` / `medium` / `high` / `highest` | `low` |
| `-video-qr-size` | Размер фрагмента QR в байтах, `0` = авто | `0` |
| `-video-tile-module` | Размер тайла в пикселях 1..270 (только `tile`) | `4` |
| `-video-tile-rs` | Reed-Solomon паритет % 0..200 (только `tile`) | `20` |
| `-ffmpeg` | Путь к исполняемому файлу ffmpeg | `ffmpeg` |

Для codec `tile` нужно точно `1080x1080`.

---

## Готовые команды

### wbstream + datachannel (рекомендуется - максимальная скорость, без бана)

```sh
# room ID нужно создать вручную через https://stream.wb.ru
ROOM_ID="<room-id-со-stream.wb.ru>"

# сервер
./olcrtc -mode srv -carrier wbstream -transport datachannel \
  -id "$ROOM_ID" -client-id <client-id> -key <hex-key> -link direct -data data -dns 1.1.1.1:53

# клиент
./olcrtc -mode cnc -carrier wbstream -transport datachannel \
  -id "$ROOM_ID" -client-id <client-id> -key <hex-key> -link direct -data data -dns 1.1.1.1:53 \
  -socks-host 127.0.0.1 -socks-port 1080
```

### wbstream + datachannel + SOCKS5 аутентификация

```sh
# клиент с логином и паролем на прокси
./olcrtc -mode cnc -carrier wbstream -transport datachannel \
  -id "$ROOM_ID" -client-id <client-id> -key <hex-key> -link direct -data data -dns 1.1.1.1:53 \
  -socks-host 127.0.0.1 -socks-port 1080 \
  -socks-user myuser -socks-pass mypass
```

Использование:
```sh
curl --socks5-hostname myuser:mypass@127.0.0.1:1080 https://icanhazip.com
# или
export all_proxy=socks5h://myuser:mypass@127.0.0.1:1080
```

---

### telemost + vp8channel

```sh
# сервер
./olcrtc -mode srv -carrier telemost -transport vp8channel \
  -id <room-id> -client-id <client-id> -key <hex-key> -link direct -data data \
  -vp8-fps 60 -vp8-batch 64

# клиент
./olcrtc -mode cnc -carrier telemost -transport vp8channel \
  -id <room-id> -client-id <client-id> -key <hex-key> -link direct -data data \
  -socks-host 127.0.0.1 -socks-port 1080 \
  -vp8-fps 60 -vp8-batch 64
```

### telemost + seichannel

```sh
# сервер
./olcrtc -mode srv -carrier telemost -transport seichannel \
  -id <room-id> -client-id <client-id> -key <hex-key> -link direct -data data \
  -fps 60 -batch 64 -frag 900 -ack-ms 2000

# клиент
./olcrtc -mode cnc -carrier telemost -transport seichannel \
  -id <room-id> -client-id <client-id> -key <hex-key> -link direct -data data \
  -socks-host 127.0.0.1 -socks-port 1080 \
  -fps 60 -batch 64 -frag 900 -ack-ms 2000
```

### telemost + videochannel (крайний случай)

```sh
# сервер
./olcrtc -mode srv -carrier telemost -transport videochannel \
  -id <room-id> -client-id <client-id> -key <hex-key> -link direct -data data \
  -video-codec qrcode -video-w 1080 -video-h 1080 \
  -video-fps 60 -video-bitrate 5000k -video-hw none

# клиент
./olcrtc -mode cnc -carrier telemost -transport videochannel \
  -id <room-id> -client-id <client-id> -key <hex-key> -link direct -data data \
  -socks-host 127.0.0.1 -socks-port 1080 \
  -video-codec qrcode -video-w 1080 -video-h 1080 \
  -video-fps 60 -video-bitrate 5000k -video-hw none
```

---

Подробнее про запуск: [Быстрый старт](fast.md) · [Мануальная сборка](manual.md)

URI-формат для клиентов: [uri.md](uri.md) · [Формат подписки](sub.md)
