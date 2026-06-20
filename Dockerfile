# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o beacon \
        ./cmd/beacon

# ── Final stage (distroless, non-root) ───────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=builder /app/beacon /beacon

EXPOSE 8080

ENTRYPOINT ["/beacon"]
