<div align="center">

<img src="https://github.com/openlibrecommunity/material/blob/master/olcrtc.png" width="250" height="250">

![License](https://img.shields.io/badge/license-WTFPL-0D1117?style=flat-square&logo=open-source-initiative&logoColor=green&labelColor=0D1117)
![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)

</div>

# Мануальная сборка

Этот способ для тех кто хочет собрать бинарник руками без Docker/Podman.
Нужен Go 1.26+, mage, git.

Проект в бете. По проблемам: t.me/openlibrecommunity

---

## Шаг 1: Установить git

```sh
apt install git       # Debian   / Ubuntu  / Mint
pacman -S git         # Arch    / CachyOS / Manjaro
dnf install git       # Fedora / RHEL   / CentOS
```

---

## Шаг 2: Установить Go 1.26+

### Arch / Fedora (всё просто)

```sh
pacman -S go    # Arch / CachyOS / Manjaro
dnf install go  # Fedora / RHEL   / CentOS
```

### Debian / Ubuntu (системный пакет устаревший)

На Debian/Ubuntu в репозитории обычно Go 1.19.

На Debian 13 лучше через `testing` c `APT Pinning`, чтобы не засорять ОС:

```sh
echo 'deb http://deb.debian.org/debian/ testing main non-free-firmware' | sudo tee /etc/apt/sources.list.d/testing.list

cat <<EOF | sudo tee /etc/apt/preferences.d/testing-pin
Package: *
Pin: release a=testing
Pin-Priority: 100
EOF

sudo apt update
sudo apt install -t testing golang-1.26

sudo update-alternatives --install /usr/bin/go go `which go` 10
sudo update-alternatives --install /usr/bin/gofmt gofmt `which gofmt` 10
sudo update-alternatives --install /usr/bin/go go /usr/lib/go-1.26/bin/go 20
sudo update-alternatives --install /usr/bin/gofmt gofmt /usr/lib/go-1.26/bin/gofmt 20
```

Иначе через SDK:

```sh
apt install golang                         # ставим старый go - он нужен только чтобы скачать новый
go install golang.org/dl/go1.26.0@latest   # скачиваем установщик go1.26
~/go/bin/go1.26.0 download                 # скачиваем сам go1.26
mv ~/go/bin/go1.26.0 /usr/local/bin/go     # заменяем системный go
```

### Проверка

```sh
go version
# go version go1.26.x linux/amd64
```

---

## Шаг 3: Установить mage

mage - система сборки для Go-проектов, аналог make.

```sh
go install github.com/magefile/mage@latest
```

Добавь `~/go/bin` в PATH:

```sh
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

Проверка:

```sh
mage --version
# mage vx.x.x
```

---

## Шаг 4: Скачать репозиторий

```sh
git clone https://github.com/openlibrecommunity/olcrtc --recurse-submodules
cd olcrtc
```

`--recurse-submodules` обязателен - без него videochannel не соберётся.

---

## Шаг 5: Собрать

```sh
mage build   # текущая платформа
mage cross   # все платформы сразу (если собираешь для другой машины)
```

Результат в `build/`:

```
build/olcrtc-linux-amd64
build/olcrtc-linux-arm64
build/olcrtc-windows-amd64.exe
build/olcrtc-darwin-amd64
```

---

## Шаг 6: Сгенерировать ключ шифрования

Делается один раз на сервере. Ключ должен совпадать на сервере и клиенте.

```sh
openssl rand -hex 32 
# d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799
```

Сохрани вывод - понадобится при запуске клиента.

---

## Шаг 7: Придумать client ID

Это обязательный идентификатор клиента. Он должен совпадать на сервере и клиенте, иначе сервер отклонит соединение.

```sh
CLIENT_ID=default
```

Подойдёт любая короткая строка без пробелов: `home-laptop`, `android-01`, `archlinux`.

---

## Шаг 8: Запустить сервер

На серверной машине (VPS и т.д.). Подбери нужную комбинацию carrier + transport из матрицы в [settings.md](settings.md).

### wbstream + datachannel (рекомендуется - максимальная скорость и пинг)

Сначала сгенерируй Room ID:

```sh
ROOM_ID=$(./build/olcrtc-linux-amd64 -mode gen -carrier wbstream -dns 1.1.1.1:53 -amount 1 -data data)
echo "Room ID: $ROOM_ID"
```

Или создай руму вручную через сайт [wbstream](https://stream.wb.ru).

Затем запусти сервер:

```sh
./build/olcrtc-linux-amd64 \
  -mode srv \
  -carrier wbstream \
  -transport datachannel \
  -id "$ROOM_ID" \
  -client-id "$CLIENT_ID" \
  -key d823fa01cb3e0609b67322f7cf984c4ee2e4ce2e294936fc24ef38c9e59f4799 \
  -link direct \
  -dns 1.1.1.1:53 \
  -data data
```

Room ID нужно передать клиенту.

### Добавить отладку

Добавь `--debug` к любой команде - увидишь каждое соединение:

```
2026/05/03 08:05:23 Connecting link via direct/datachannel/wbstream...
2026/05/03 08:05:25 wbstream publisher state: connected
2026/05/03 08:05:27 Link connected
2026/05/03 08:05:43 sid=3 connect icanhazip.com:443
2026/05/03 08:05:43 sid=3 connected icanhazip.com
```

---

## Шаг 9: Запустить клиент

На своей машине. Carrier, transport, id, `client-id` и key должны совпадать с сервером.

### wbstream + datachannel

```sh
./build/olcrtc-linux-amd64 \
  -mode cnc \
  -carrier wbstream \
  -transport datachannel \
  -id abc123xyz \
  -client-id "$CLIENT_ID" \
  -key <hex-key> \
  -link direct \
  -dns 1.1.1.1:53 \
  -data data \
  -socks-host 127.0.0.1 \
  -socks-port 1080
```

После старта в логах появится:

```
SOCKS5 server listening on 127.0.0.1:1080
```

---

## Шаг 10: Проверить

```sh
curl --socks5-hostname 127.0.0.1:1080 https://icanhazip.com
```

Должен вернуть IP сервера.

Или выставить переменную чтобы весь трафик шёл через прокси:

```sh
export all_proxy=socks5h://127.0.0.1:1080
curl https://icanhazip.com
```

---

## Все mage таргеты

```sh
mage build    # собрать для текущей платформы
mage cross    # собрать для всех платформ
mage deps     # скачать и обновить зависимости
mage clean    # удалить build/
mage test     # запустить тесты
mage lint     # запустить линтер
mage podman   # собрать образ через podman
mage docker   # собрать образ через docker
```

---

Используешь скрипты вместо ручной сборки? -> [Быстрый старт](fast.md)

Все флаги и матрица совместимости -> [settings.md](settings.md)
