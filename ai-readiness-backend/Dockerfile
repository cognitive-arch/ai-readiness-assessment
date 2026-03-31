# ── Build stage ──────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /app/bin/server ./cmd/server

RUN mkdir -p /tmp/ai-readiness-pdfs && chown -R 65534:65534 /tmp/ai-readiness-pdfs

# ── Runtime stage ─────────────────────────────────────────────
FROM scratch

# Import TLS certs and timezone data from the builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

COPY --from=builder /app/bin/server ./server

# Copy question bank (required at runtime)
COPY question-bank-v1.json ./question-bank-v1.json

#Copy .env file (required at runtime)
COPY .env .env

# PDF temp dir — writable
COPY --from=builder /tmp/ai-readiness-pdfs /tmp/ai-readiness-pdfs

EXPOSE 8080

ENTRYPOINT ["./server"]
