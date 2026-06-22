FROM golang:1.22.2-alpine AS builder

WORKDIR /src
ARG APP_VERSION=2.2.0-latest
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${APP_VERSION}" -o /out/sublinkX .

FROM alpine:3.20

ARG APP_VERSION=latest

LABEL org.opencontainers.image.title="sublinkX" \
      org.opencontainers.image.description="sublinkX subscription manager with Clash Verge/Mihomo xhttp support" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.source="https://github.com/wangxvwei/sublinkX"

WORKDIR /app

ENV TZ=Asia/Shanghai \
    GIN_MODE=release \
    UPDATE_REPO=wangxvwei/sublinkX \
    DOCKER_IMAGE=ghcr.io/wangxvwei/sublinkx

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S sublink \
    && adduser -S -G sublink sublink \
    && mkdir -p /app/db /app/logs /app/template \
    && chown -R sublink:sublink /app

COPY --from=builder /out/sublinkX /app/sublinkX

USER sublink
EXPOSE 8000
VOLUME ["/app/db", "/app/logs", "/app/template"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8000/api/v1/version >/dev/null || exit 1

CMD ["/app/sublinkX"]
