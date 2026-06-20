# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# Install CA certs so the binary can make TLS calls (e.g. Telegram).
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 1001 beacon

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o beacon \
        ./cmd/beacon

# ── Final stage (scratch, non-root) ──────────────────────────────────────────
FROM scratch AS final

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/beacon /beacon

USER beacon
EXPOSE 8080
ENTRYPOINT ["/beacon"]
