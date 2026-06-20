BINARY  := beacon
GOFLAGS := CGO_ENABLED=0

.PHONY: run build test lint tidy docker-up docker-down migrate-up migrate-down gen

## run: start the service locally (requires BEACON_DB_DSN env var)
run:
	go run ./cmd/beacon

## build: produce a static binary in ./bin/
build:
	$(GOFLAGS) go build -trimpath -ldflags="-s -w" -o bin/$(BINARY) ./cmd/beacon

## test: run all tests with race detector and coverage
test:
	go test -race -count=1 -coverprofile=coverage.out ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## tidy: sync go.mod / go.sum
tidy:
	go mod tidy

## docker-up: build and start all services
docker-up:
	docker compose -f deployments/docker-compose.yml up --build -d

## docker-down: stop services and remove volumes
docker-down:
	docker compose -f deployments/docker-compose.yml down -v

## migrate-up: apply pending migrations (requires BEACON_DB_DSN)
migrate-up:
	goose -dir migrations postgres "$(BEACON_DB_DSN)" up

## migrate-down: rollback last migration batch
migrate-down:
	goose -dir migrations postgres "$(BEACON_DB_DSN)" down

## gen: regenerate mocks (requires mockgen)
gen:
	go generate ./...
