.DEFAULT_GOAL := help
.PHONY: help build run tidy fmt lint test test-race test-cover e2e e2e-perf \
        docker-build up down logs ps infra-up infra-down

E2E_PORT ?= 8090

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

# ---- End-to-end (black-box, godog) ----
## e2e: boot the server and run the BLOCKING functional e2e gate (Tier 1)
e2e:
	E2E_TAGS='~@perf' E2E_PORT=$(E2E_PORT) ./scripts/e2e.sh

## e2e-perf: run the ADVISORY e2e tier (@perf) — never fails the build
e2e-perf:
	@E2E_TAGS='@perf' E2E_PORT=$(E2E_PORT) ./scripts/e2e.sh || \
	  echo "⚠️  e2e-perf: advisory failures (non-blocking) — see output above"

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
