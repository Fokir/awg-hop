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

Собранные multi-arch (`linux/amd64`, `linux/arm64`) образы публикуются на каждый push в `main` и на каждый тег `v*` в `ghcr.io/fokir/awg-hop`. Поэтому установка сводится к скачиванию compose-файла и `docker compose up -d`.

Выберите один из двух сценариев ниже.

---

### Сценарий A. Локальная установка (HTTP на 127.0.0.1)

Подходит для тестирования или когда у вас уже есть внешний reverse proxy.

**Требования:** Linux-хост с публичным IP, Docker 24+ с плагином `compose`, открытый UDP `51820`.

1. **Подготовьте каталог и compose-файл:**

   ```bash
   mkdir awghop && cd awghop
   curl -fLO https://raw.githubusercontent.com/Fokir/awg-hop/main/docker-compose.standalone.yml
   ```

2. **Поднимите контейнер:**

   ```bash
   docker compose -f docker-compose.standalone.yml up -d
   ```

3. **Откройте админку и пройдите мастер:** `http://127.0.0.1:8080` →
   * задайте пароль администратора;
   * укажите публичный endpoint AmneziaWG (`<публичный_IP_хоста>:51820` или DNS-имя);
   * при наличии `wg-easy` экспорта с включённым AmneziaWG — загрузите `wg0.json`.

4. **Создайте первого клиента:** вкладка **Клиенты → + Новый клиент**, скачайте `.conf` или QR. Подключитесь с телефона/ПК, проверьте, что трафик идёт.

5. **(Опционально) Добавьте upstream:** вкладка **Upstream-подключения → + Новый upstream**, вставьте `.conf` для `awg-quick` от удалённого AWG-сервера. Затем у клиента смените маршрут на «через upstream».

6. **Применить изменения:** кнопка **«Применить»** на Dashboard (или `POST /api/v1/system/apply`) — поднимет интерфейсы, расставит `ip rule`/`iptables MASQUERADE`.

> Файрвол: убедитесь, что UDP `51820` открыт из интернета. HTTP `8080` биндится только на `127.0.0.1`.

---

### Сценарий B. Production: HTTPS через Caddy + Let's Encrypt

Подходит для боевого развёртывания: автоматический TLS, заголовки безопасности (HSTS/CSP/X-Frame-Options), аккуратный reverse proxy.

**Требования:** Linux-хост с публичным IP, Docker 24+, домен с A/AAAA-записью на этот IP, открытые порты `80/tcp`, `443/tcp`, `51820/udp`.

1. **Подготовьте каталог:**

   ```bash
   mkdir awghop && cd awghop
   ```

2. **Скачайте compose и Caddyfile:**

   ```bash
   curl -fLO https://raw.githubusercontent.com/Fokir/awg-hop/main/docker-compose.example.yml
   curl -fLO https://raw.githubusercontent.com/Fokir/awg-hop/main/Caddyfile
   mv docker-compose.example.yml docker-compose.yml
   ```

3. **Создайте `.env` с вашим доменом и e-mail для ACME:**

   ```bash
   cat > .env <<'EOF'
   AWGHOP_DOMAIN=awghop.example.com
   AWGHOP_LE_EMAIL=admin@example.com
   AWGHOP_IMAGE_TAG=latest
   EOF
   ```

   Для воспроизводимости лучше зафиксировать тег: `AWGHOP_IMAGE_TAG=v0.2.0` (см. [GitHub Releases](https://github.com/Fokir/awg-hop/releases)).

4. **Проверьте сетевые требования:**

   * `dig +short awghop.example.com` возвращает публичный IP хоста (нужно для ACME http-01);
   * `ufw`/cloud security-group разрешают входящие `80/tcp`, `443/tcp`, `51820/udp`;
   * `net.ipv4.ip_forward=1` на хосте (compose выставляет его в самом контейнере, но если у вас кастомный sysctl на хосте — проверьте).

5. **Поднимите стек:**

   ```bash
   docker compose up -d
   docker compose logs -f caddy   # дождитесь "certificate obtained successfully"
   ```

   Caddy выпустит TLS-сертификат и начнёт проксировать `https://awghop.example.com → awghop:8080`.

6. **Откройте админку и пройдите мастер:** `https://awghop.example.com` — те же шаги, что в сценарии A (пароль, endpoint, клиенты, upstream'ы, Apply).

7. **(Рекомендуется) Сделайте первый бэкап:** вкладка **Бэкап / Импорт → Скачать бэкап** — сохраните `.zip` куда-то вне хоста.

---

### Если порт 51820 занят (другой WireGuard / wg-easy на хосте)

Симптом — `Bind for 0.0.0.0:51820 failed: port is already allocated` при `docker compose up`.

Найдите, кто держит порт:

```bash
sudo ss -ulpn | grep 51820
docker ps --filter publish=51820
```

Дальше один из двух вариантов:

* **Освободить 51820** (если это старый wg-easy, который вы заменяете на AWG Hop):

  ```bash
  docker stop wg-easy && docker rm wg-easy           # если wg-easy в Docker
  sudo systemctl stop  wg-quick@wg0                  # если нативный wg на хосте
  sudo systemctl disable wg-quick@wg0
  docker compose up -d
  ```

* **Поднять AWG Hop на другом UDP-порте**, не трогая существующий сервис. В `.env` пропишите `AWGHOP_PUBLIC_UDP_PORT` и перепустите:

  ```bash
  echo 'AWGHOP_PUBLIC_UDP_PORT=51821' >> .env
  docker compose up -d
  ```

  Внутри контейнера AmneziaWG продолжает слушать `51820/udp` — меняется только наружный маппинг. После запуска зайдите в админку → **Настройки → Входной AmneziaWG** и в `Public Endpoint` укажите `<публичный_IP_или_домен>:51821`, нажмите **Сохранить**, затем **Применить** на Dashboard. Все вновь сгенерированные клиентские `.conf` будут содержать новый endpoint.

### Обновление и закрепление версии

```bash
docker compose pull && docker compose up -d
```

Volume `awghop-data` сохраняется между апгрейдами. Перед минорным обновлением скачайте бэкап (вкладка *Бэкап / Импорт*).

Чтобы зафиксироваться на конкретной версии — пропишите в `.env`:

```env
AWGHOP_IMAGE_TAG=v0.2.0
```

Список доступных тегов — на [GHCR](https://github.com/Fokir/awg-hop/pkgs/container/awg-hop) и в [Releases](https://github.com/Fokir/awg-hop/releases).

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
