.DEFAULT_GOAL := help
.PHONY: help build run tidy fmt lint test test-race test-cover \
        docker-build up down logs ps infra-up infra-down

## help: list available commands
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F': ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# ---- Build / run (native — Mode B) ----
## build: compile the server binary to ./bin/quiz
build:
	go build -o bin/quiz ./cmd/server

## run: run the server natively (reads .env vars from your shell)
run:
	go run ./cmd/server

## tidy: tidy go.mod / go.sum
tidy:
	go mod tidy

# ---- Quality ----
## fmt: format the code
fmt:
	gofmt -w .

## lint: run golangci-lint
lint:
	golangci-lint run ./...

# ---- Test ----
## test: run all tests
test:
	go test ./...

## test-race: run all tests with the race detector
test-race:
	go test -race ./...

## test-cover: run tests with coverage summary
test-cover:
	go test -cover ./internal/... ./pkg/...

# ---- Full Docker stack (Mode A) ----
## docker-build: build the production image
docker-build:
	docker build -t elsa-quiz:latest .

## up: start server + Prometheus + Grafana, all in Docker
up:
	docker compose --profile full up --build -d

## down: stop the full Docker stack
down:
	docker compose --profile full down

## logs: tail logs from the full stack
logs:
	docker compose --profile full logs -f

## ps: show running compose services
ps:
	docker compose ps

# ---- Infra only (Mode B helper) ----
## infra-up: start ONLY Prometheus + Grafana (run the server with `make run`)
infra-up:
	docker compose up -d prometheus grafana

## infra-down: stop Prometheus + Grafana
infra-down:
	docker compose down
