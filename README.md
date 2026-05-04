# AWG Hop

Админ-панель для входного **AmneziaWG**: создаёте клиентов (как в `wg-easy`), а наш сервер дополнительно умеет сам быть **AWG-клиентом** одного или нескольких удалённых AWG-серверов. Каждого клиента можно направить либо в интернет напрямую, либо через выбранный upstream-туннель.

```
устройство-клиент ─AWG─▶ AWG Hop (вход) ─AWG-клиент─▶ удалённый AWG-сервер ─▶ интернет
```

Стек: **Go** (backend), **Svelte** (`web/` → `internal/ui/dist`), Docker.

[![CI](https://github.com/Fokir/awg-hop/actions/workflows/ci.yml/badge.svg)](https://github.com/Fokir/awg-hop/actions/workflows/ci.yml)
[![Release](https://github.com/Fokir/awg-hop/actions/workflows/release.yml/badge.svg)](https://github.com/Fokir/awg-hop/actions/workflows/release.yml)
[![ghcr.io](https://img.shields.io/badge/ghcr.io-fokir%2Fawg--hop-blue)](https://github.com/Fokir/awg-hop/pkgs/container/awg-hop)

Спецификация продукта: [docs/SPECIFICATION.md](docs/SPECIFICATION.md). REST-схема: [docs/openapi.yaml](docs/openapi.yaml).

## Быстрый старт (Docker Compose)

Собранные multi-arch (`linux/amd64`, `linux/arm64`) образы публикуются на каждый push в `main` и на каждый тег `v*` в `ghcr.io/fokir/awg-hop`.

### 1. Минимальная установка (HTTP на localhost)

```bash
mkdir awghop && cd awghop
curl -fLO https://raw.githubusercontent.com/Fokir/awg-hop/main/docker-compose.standalone.yml

docker compose -f docker-compose.standalone.yml up -d
```

После старта откройте `http://127.0.0.1:8080` и пройдите мастер первого запуска. AmneziaWG слушает UDP `51820` снаружи; данные хранятся в docker-volume `awghop-data`.

> Используйте этот режим только для локального тестирования или за внешним reverse proxy. Для прода — следующий пункт.

### 2. Production: HTTPS через Caddy + Let's Encrypt

```bash
mkdir awghop && cd awghop
curl -fLO https://raw.githubusercontent.com/Fokir/awg-hop/main/docker-compose.example.yml
curl -fLO https://raw.githubusercontent.com/Fokir/awg-hop/main/Caddyfile
mv docker-compose.example.yml docker-compose.yml

cat > .env <<'EOF'
AWGHOP_DOMAIN=awghop.example.com
AWGHOP_LE_EMAIL=admin@example.com
AWGHOP_IMAGE_TAG=latest
EOF

docker compose up -d
```

Caddy сам получит TLS-сертификат и будет проксировать `https://awghop.example.com → awghop:8080` с HSTS/CSP/X-Frame-Options.

Убедитесь, что:
* DNS `AWGHOP_DOMAIN` указывает на хост (для ACME http-01),
* открыты порты `80/tcp`, `443/tcp` (для Let's Encrypt и UI) и `51820/udp` (для AmneziaWG),
* в фаерволе/cloud security-group выставлены те же правила.

### 3. Закрепить версию

В `.env` укажите `AWGHOP_IMAGE_TAG=v0.2.0` (или другой опубликованный тег). Список тегов — на [GitHub Releases](https://github.com/Fokir/awg-hop/releases) или странице GHCR.

### 4. Обновление

```bash
docker compose pull && docker compose up -d
```

Volume `awghop-data` сохраняется между обновлениями. Перед минорным апгрейдом рекомендуется зайти в админку и нажать **«Скачать бэкап»** на вкладке *Бэкап / Импорт*.

## Что умеет

* Bootstrap-мастер: пароль администратора, параметры входного AmneziaWG (Jc/Jmin/Jmax/S1..S4/H1..H4), опциональный импорт `wg-easy/wg0.json` с AmneziaWG-блоком.
* CRUD клиентов с генерацией ключей, AllowedIPs-аллокацией, экспортом `.conf` и QR-кодом.
* CRUD upstream-подключений (наш сервер выступает AWG-клиентом удалённого сервера) с редактором `.conf` для `awg-quick`.
* Per-client маршрут: `direct` (NAT в интернет контейнера) или `via_upstream → <upstream-туннель>`.
* `POST /system/apply`: подъём интерфейсов через `awg-quick`, policy routing (`ip rule`/`ip route`) и **iptables MASQUERADE** per-client на нужном внешнем/upstream-интерфейсе. Состояние сохраняется и атомарно откатывается.
* `awg show <iface> dump` парсится — handshake / RX / TX отображаются для клиентов и upstream-туннелей в UI и `GET /system/status`.
* Бэкап: zip с БД и `manifest.json`; импорт восстанавливает БД (с `.bak`-файлом) и автоматически делает Apply.
* CSRF (двойная cookie), rate-limit на `/auth/login`, secure-cookies при `AWGHOP_TLS=1`.
* Структурированные логи `log/slog` (text по умолчанию, `AWGHOP_LOG_FORMAT=json` для сборщиков логов).
* Auto-Apply при старте (`AWGHOP_AUTO_APPLY=1` по умолчанию).

## Запуск (разработка)

Бэкенд:

```bash
go run ./cmd/awghop
```

Фронтенд (Svelte + Vite):

```bash
cd web && npm install && npm run build
```

Для разработки UI с прокси на API: `cd web && npm run dev` (ожидает API на `http://127.0.0.1:8080`). Для cross-origin dev запустите сервер с `AWGHOP_DEV=1`.

По умолчанию HTTP `:8080`, данные в `./data/awghop.db`.

### Переменные окружения

| Переменная | Назначение | По умолчанию |
|---|---|---|
| `AWGHOP_LISTEN` | адрес HTTP-сервера | `:8080` |
| `AWGHOP_DATA` | каталог данных (БД, ключи, конфиги) | `./data` |
| `AWGHOP_DATABASE` | путь к SQLite | `$AWGHOP_DATA/awghop.db` |
| `AWGHOP_DEV` | dev-CORS (любой Origin) | `0` |
| `AWGHOP_ALLOWED_ORIGINS` | csv-list разрешённых Origin для prod | пусто |
| `AWGHOP_TLS` / `AWGHOP_SECURE_COOKIES` | secure-cookies (за reverse proxy с TLS) | `0` |
| `AWGHOP_WG_QUICK_BIN` | бинарник wg-quick | `wg-quick` (`awg-quick` в Docker) |
| `AWGHOP_AWG_BIN` | бинарник `awg`/`wg` для статуса | `awg` |
| `AWGHOP_IPTABLES_BIN` | iptables/iptables-nft | `iptables` |
| `AWGHOP_IP_BIN` | бинарник `ip` (iproute2) | `ip` |
| `AWGHOP_EXTERNAL_IFACE` | внешний интерфейс для NAT direct-клиентов | автоопределение |
| `AWGHOP_AUTO_APPLY` | вызывать `Apply` при старте | `1` |
| `AWGHOP_LOG_LEVEL` | `debug`/`info`/`warn`/`error` | `info` |
| `AWGHOP_LOG_FORMAT` | `text` или `json` | `text` |

## Образ Docker

Образ ставит в runtime **`awg`**, **`awg-quick`** ([amneziawg-tools](https://github.com/amnezia-vpn/amneziawg-tools)) и **`amneziawg-go`** ([amneziawg-go](https://github.com/amnezia-vpn/amneziawg-go)). По умолчанию оба собираются из ветки `master`. Для воспроизводимых релизов передайте конкретный ref (тег или sha) через `--build-arg AWG_TOOLS_REF=…` / `AWG_GO_REF=…`.

Если в контейнере нет модуля ядра `amneziawg`, `awg-quick` поднимает интерфейс через userspace (`WG_QUICK_USERSPACE_IMPLEMENTATION=/usr/local/bin/amneziawg-go`), для этого compose пробрасывает `/dev/net/tun`.

Локальная сборка из исходников (без публикации):

```bash
docker compose up --build      # использует docker-compose.yml в репо (HTTP на 127.0.0.1:8080)
```

Сборка multi-arch (amd64 + arm64) автоматически выкатывается GitHub Actions из workflow `release.yml` на каждый push в `main` (`:latest`, `:main`, `:sha-<short>`) и на тег `v*` (`:v0.2.0`, `:0.2`, `:0`).

### Capabilities / sysctls

* `CAP_NET_ADMIN` — обязательно;
* `net.ipv4.ip_forward=1` — выставляется в compose;
* `/dev/net/tun` — нужен для userspace `amneziawg-go`;
* публикация UDP-порта входа AmneziaWG (51820).

## Применение конфигурации

`POST /api/v1/system/apply` (или кнопка «Применить» в Dashboard) делает:

1. Сносит предыдущие `iptables`/`ip rule`/`ip route` по сохранённому state.
2. Опускает интерфейсы AmneziaWG по списку из `wireguard-runtime-state.json`.
3. Перегенерирует `$AWGHOP_DATA/wireguard/<iface>.conf` (вход) и `upstream-<id>-*.conf` (наши исходящие подключения) и поднимает их `awg-quick up`.
4. Расставляет `ip rule from <client>/32 → table 10000+upstream_tunnel_id` и `ip route replace default dev <upstream_iface> table …`.
5. Расставляет `iptables -t nat -A POSTROUTING -s <client>/32 -o <iface> -j MASQUERADE`:
   * для `direct` — на внешний интерфейс контейнера (по `AWGHOP_EXTERNAL_IFACE`/`system_settings.external_interface` или autodetect через `ip route get 1.1.1.1`);
   * для `via_upstream` — на интерфейс выбранного upstream-туннеля.
6. Сохраняет state на диск.

Политика недоступного upstream-туннеля — `system_settings.tunnel_offline_policy`:
* `block` (по умолчанию, согласно §6.4) — `Apply` падает, если у клиента выбран отсутствующий/выключенный upstream;
* `ignore` — клиент пропускается, остальной конфиг применяется.

## Безопасность

* httpOnly-сессионная cookie `awghop_session` + CSRF (`X-CSRF-Token` против `awghop_csrf` cookie, double-submit).
* Rate-limit на `/auth/login` и `/setup/bootstrap` (5 попыток / минута / IP).
* В production используйте `AWGHOP_TLS=1` и эталонный compose с Caddy.

## Тесты

```bash
go test ./...
```

Юнит-тесты есть для парсера `awg show dump` и парсера wg-easy экспорта.
