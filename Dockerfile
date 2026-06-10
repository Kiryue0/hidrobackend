# ── Derleme (build) aşaması ──────────────────────────────────────────────
FROM golang:1.25-alpine AS build

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

# ── Çalışma (runtime) aşaması ────────────────────────────────────────────
FROM alpine:3.21

# Sistem kök sertifikaları (MQTT TLS / HiveMQ Cloud için gerekli)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Binary
COPY --from=build /bin/server /app/server

# Migration'lar (başlangıçta otomatik uygulanır)
COPY migrations/ /app/migrations/

# Web UI (tek sayfa)
COPY web/ /app/web/

# Railway $PORT enjekte eder; varsayılan 8080
ENV HTTP_PORT=8080 \
    MIGRATIONS_DIR=/app/migrations \
    WEB_DIR=/app/web

EXPOSE 8080

CMD ["/app/server"]
