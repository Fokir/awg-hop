# syntax=docker/dockerfile:1.7

# Версии apстрима AmneziaWG, известные совместимые с этим релизом AWG Hop.
# Если нужно обновиться — поднимите эти переменные сборки осознанно и протестируйте.
ARG AWG_TOOLS_REF=v1.0.20240705
ARG AWG_GO_REF=v0.2.12

# UI -> internal/ui/dist
FROM node:22-bookworm AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# AmneziaWG userspace (fallback when kernel module amneziawg is absent — typical in Docker).
FROM golang:1.24-bookworm AS awg-go-build
ARG AWG_GO_REF
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /build
RUN git clone https://github.com/amnezia-vpn/amneziawg-go.git \
  && cd amneziawg-go \
  && git -c advice.detachedHead=false checkout "$AWG_GO_REF" \
  && make \
  && make install DESTDIR=/staging PREFIX=/usr

# awg / awg-quick from amneziawg-tools.
FROM debian:bookworm-slim AS awg-tools-build
ARG AWG_TOOLS_REF
RUN apt-get update && apt-get install -y --no-install-recommends \
    git make gcc libc6-dev bash ca-certificates \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /build
RUN git clone https://github.com/amnezia-vpn/amneziawg-tools.git \
  && cd amneziawg-tools \
  && git -c advice.detachedHead=false checkout "$AWG_TOOLS_REF" \
  && cd src \
  && make -j"$(nproc)" \
  && make install DESTDIR=/staging WITH_WGQUICK=yes

# Go-бинарник (modernc SQLite — без CGO).
FROM golang:1.22-bookworm AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/internal/ui/dist ./internal/ui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /awghop ./cmd/awghop

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates bash iproute2 iptables nftables tini \
  && rm -rf /var/lib/apt/lists/*
COPY --from=awg-tools-build /staging/usr/bin/awg /staging/usr/bin/awg-quick /usr/local/bin/
COPY --from=awg-go-build /staging/usr/bin/amneziawg-go /usr/local/bin/
WORKDIR /app
COPY --from=gobuild /awghop /app/awghop
ENV AWGHOP_LISTEN=:8080
ENV AWGHOP_DATA=/data
ENV AWGHOP_WG_QUICK_BIN=awg-quick
ENV AWGHOP_AWG_BIN=awg
ENV WG_QUICK_USERSPACE_IMPLEMENTATION=/usr/local/bin/amneziawg-go
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/bin/tini", "--", "/app/awghop"]
