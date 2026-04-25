# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata build-base

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/strm-cas ./cmd/strm-cas && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/strm-cas-api ./cmd/strm-cas-api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata bash
WORKDIR /app
COPY --from=builder /out/strm-cas /usr/local/bin/strm-cas
COPY --from=builder /out/strm-cas-api /usr/local/bin/strm-cas-api

ENV TZ=Asia/Shanghai \
    STRM_CAS_LISTEN=:18457 \
    STRM_CAS_STRM_ROOT=/strm \
    STRM_CAS_CACHE_DIR=/cache \
    STRM_CAS_DOWNLOAD_DIR=/download \
    STRM_CAS_DB_PATH=/download/strm-cas.db \
    STRM_CAS_LOG_PATH=/download/strm-cas-summary.json

EXPOSE 18457
VOLUME ["/strm", "/cache", "/download"]

CMD ["strm-cas-api"]
