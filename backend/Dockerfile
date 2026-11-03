# syntax=docker/dockerfile:1.7

FROM alpine:3.22 AS protocol-package-build

RUN apk add --no-cache nodejs zip

WORKDIR /src
COPY plugin-packages ./plugin-packages
RUN ./plugin-packages/build-packages.sh

# 助手宿主用 http.setGlobalProxyFromEnv 接管代理，该 API 只有 Node 24 起提供，
# alpine 仓库的 nodejs 仍是 22，因此单独取一份官方 musl 运行时。
FROM node:24-alpine AS node-runtime

# 内置助手宿主：运行镜像不自带 Node，这里把 agent-host 与依赖单独装好再一起打包。
FROM oven/bun:1.4.2 AS agent-host-build

WORKDIR /app/agent-host
COPY agent-host/package.json agent-host/bun.lock ./
RUN bun install --frozen-lockfile --omit=dev --cache-dir=/root/.bun/install/cache \
	&& rm -rf /root/.bun/install/cache
COPY agent-host ./

FROM golang:1.25-alpine AS backend-build

ARG GOPROXY=https://proxy.golang.org,direct
ARG BUILD_VERSION
ARG BUILD_COMMIT=unknown
ARG BUILD_TIME=unknown
ENV GOPROXY=$GOPROXY

RUN apk add --no-cache build-base

WORKDIR /src/backend
COPY VERSION /src/VERSION
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend ./
RUN --mount=type=cache,target=/root/.cache/go-build set -eu; \
    version="${BUILD_VERSION:-$(tr -d '\r\n' </src/VERSION)}"; \
    ldflags="-s -w -X infinite-canvas/backend/internal/buildinfo.Version=${version} -X infinite-canvas/backend/internal/buildinfo.Commit=${BUILD_COMMIT} -X infinite-canvas/backend/internal/buildinfo.BuildTime=${BUILD_TIME}"; \
    CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="$ldflags" -o /out/infinite-canvas-backend ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata wget libstdc++ \
    && addgroup -S app \
    && adduser -S app -G app

WORKDIR /app
ENV GIN_MODE=release
# Loopback inside the container. Compose sets CANVAS_BACKEND_ADDR=:8080 so other
# containers can reach the API. Do not publish 8080 on all host interfaces:
# the server profile has no launch token.
ENV CANVAS_BACKEND_ADDR=127.0.0.1:8080
ENV CANVAS_BACKEND_DATA_DIR=/data

RUN mkdir -p /data && chown -R app:app /data
COPY --from=backend-build /out/infinite-canvas-backend /usr/local/bin/infinite-canvas-backend
COPY --from=agent-host-build /app/agent-host /usr/local/bin/agent-host
COPY --from=node-runtime /usr/local/bin/node /usr/local/bin/node
# BundledConfig 在可执行文件同级找 agent-host/runtime/bin/node；指向随镜像的 Node 24。
RUN mkdir -p /usr/local/bin/agent-host/runtime/bin \
    && ln -sf /usr/local/bin/node /usr/local/bin/agent-host/runtime/bin/node
COPY --from=protocol-package-build /src/plugin-packages /app/plugin-packages

USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -qO- http://127.0.0.1:8080/api/health/ready >/dev/null || exit 1
CMD ["infinite-canvas-backend"]
