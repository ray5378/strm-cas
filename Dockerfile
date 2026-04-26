FROM golang:1.25-alpine3.22 AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata

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
    STRM_CAS_STRM_ROOT=/data/strm \
    STRM_CAS_CACHE_DIR=/data/cache \
    STRM_CAS_DOWNLOAD_DIR=/data/download \
    STRM_CAS_DB_PATH=/data/strm-cas.db \
    STRM_CAS_LOG_PATH=/data/strm-cas-summary.json

EXPOSE 18457
VOLUME ["/data"]

CMD ["strm-cas-api"]
