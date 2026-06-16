# Real-Time Vocabulary Quiz

A real-time, multiplayer vocabulary quiz backend for an English-learning app.
Players join a session with a quiz ID, answer vocabulary questions, and watch a
live leaderboard update as scores change — all over WebSocket.

This repository contains the **system design, the test specification, and a
test-driven Go implementation** of the core server component. It was built for
the Real-Time Quiz coding challenge (see [`test.md`](./test.md)).

---

## What's here

| Area | Location | Status |
|------|----------|--------|
| Product requirements | [`docs/01-product-requirements.md`](./docs/01-product-requirements.md) | ✅ |
| Test cases (Gherkin / BDD) | [`docs/02-test-cases.md`](./docs/02-test-cases.md) | ✅ |
| System architecture & ADRs | [`docs/03-architecture.md`](./docs/03-architecture.md) | ✅ |
| Implementation plan (TDD) | [`docs/04-implementation-plan.md`](./docs/04-implementation-plan.md) | ✅ |
| **Backend implementation guide** | [`docs/backend-implementation/`](./docs/backend-implementation/) | ✅ |
| **API / Postman collection** | [`docs/postman/`](./docs/postman/) | ✅ |
| Go source (domain + store + service) | [`internal/`](./internal), [`pkg/`](./pkg) | ✅ |
| HTTP/REST + WebSocket layer | [`internal/handler/`](./internal/handler) | ✅ |
| Server entry point (slog, graceful shutdown) | [`cmd/server/`](./cmd/server) | ✅ |
| Observability (Prometheus `/metrics`, pprof) | [`internal/handler/metrics.go`](./internal/handler/metrics.go) | ✅ |
| Docker, docker-compose, Makefile | `Dockerfile`, `docker-compose.yml`, `Makefile` | ✅ |
| **Black-box E2E (godog, vs. a running server)** | [`features/`](./features), [`tests/e2e/`](./tests/e2e), [`docs/05-e2e-test-plan.md`](./docs/05-e2e-test-plan.md) | ✅ |
| E2E CI pipeline (blocking + advisory) | [`.github/workflows/e2e.yml`](./.github/workflows/e2e.yml) | ✅ |

> New here? Start with this README, then read
> [`docs/backend-implementation/README.md`](./docs/backend-implementation/README.md)
> for a step-by-step walkthrough of how the backend works.

---

## Features (implemented core)

- **Quiz sessions** — create a session with a unique ID; join by ID; multi-user
  participation; late-join support.
- **Real-time scoring** — answers validated and scored instantly; accurate and
  consistent (identical inputs → identical scores; no retroactive changes).
- **Live leaderboard** — deterministic ranking by `score`, tie-broken by *who
  reached the score first*.
- **Two end policies** — `manual` (host advances/ends) and `timed` (per-question
  timer) so a single AFK player can never stall the room.
- **Concurrency-safe** — the session aggregate guards its own state; verified
  race-free under 100 simultaneous submissions.

---

## Tech stack

| Concern | Choice |
|--------|--------|
| Language | Go 1.22+ (developed on 1.26) |
| Real-time transport | WebSocket (`gorilla/websocket`) |
| Persistence | In-memory behind a `Store` interface (swap for Redis/PG later) |
| Architecture | Clean/layered: `domain` → `store` → `service` → `handler` |
| Observability | Prometheus (`client_golang`) `/metrics`, pprof, structured `slog` |
| Testing | `testing` + table-driven tests, `-race`, real-WebSocket integration tests |
| Tooling | `golangci-lint`, Makefile, multi-stage Docker + docker-compose |

---

## Quick start

**Prerequisites:** Go 1.22+ (developed on 1.26) for native mode; Docker + Docker
Compose for the container modes; `make`.

```bash
cp .env.example .env
```

> **Host ports are configurable** via `.env` — `PORT` (quiz), `PROMETHEUS_PORT`,
> `GRAFANA_PORT`. If one is already taken on your machine (e.g. another Grafana on
> `3000`), set a free value there to avoid a "port is already allocated" error;
> container-internal ports never change.

**Option A — everything in Docker** (no Go toolchain needed):
```bash
make up        # server :8080 + Prometheus :9090 + Grafana :3000
curl localhost:8080/api/health
make down
```

**Option B — Go server natively, infra in Docker** (fast iteration):
```bash
make infra-up  # Prometheus + Grafana only
make run       # server on :8080
make infra-down
```

**Run the tests** (unit + real-WebSocket integration):
```bash
make test          # all packages green
make test-race     # no data races
make lint          # 0 issues
make test-cover    # coverage report
```

Coverage: domain **93.7%**, service **93.9%**, store **100%**, handler **~82%**.

**Run the end-to-end gate** (black-box: builds + boots the server, drives it with
real WebSocket + HTTP clients via `godog`, tears it down):
```bash
make e2e        # BLOCKING gate — 43/43 Tier-1 scenarios must pass
make e2e-perf   # ADVISORY — @perf scenarios; warns, never blocks
```

Run it against a server you started yourself (verbose, or a single feature, or a
remote target — no orchestration):
```bash
go build -o bin/quiz ./cmd/server && PORT=8090 ./bin/quiz &
E2E_BASE_URL=http://localhost:8090 E2E_WS_URL=ws://localhost:8090 \
  E2E_TAGS='~@perf' go test -tags e2e -v ./tests/e2e      # all Tier-1, verbose
# one feature: add  -godog.paths ../../features/leaderboard.feature
# staging:     E2E_BASE_URL=https://… E2E_WS_URL=wss://…  go test -tags e2e ./tests/e2e
```

| Env var | Default | Purpose |
|---------|---------|---------|
| `E2E_BASE_URL` | `http://localhost:8080` | REST base URL of the target |
| `E2E_WS_URL` | `ws://localhost:8080` | WebSocket base URL of the target |
| `E2E_TAGS` | *(all)* | godog tag expression (`~@perf` blocking, `@perf` advisory) |
| `E2E_PORT` | `8090` | port the `make` targets run the server on |
| `GODOG_FORMAT` | `pretty` | godog output format |

**Design notes.** The harness is deliberately **black-box**: it never imports
`internal/…` and re-declares the JSON contract locally (`tests/e2e/wire.go`), so an
accidental server-side rename surfaces as a failing scenario. godog was chosen over
a hand-rolled Go suite (the spec is *already* Gherkin — keeps `features/` 1:1 with
`docs/02`) and over Newman/Postman (can't meaningfully assert WebSocket broadcasts
or concurrency). Building the suite also surfaced a real server bug — a late answer
to an *expired* timed question returned `question_not_found` instead of `time_up`;
fixed in `internal/domain` (TDD), which let that scenario become a deterministic
blocking test. Full design, the tier/gate policy, the 47-scenario coverage matrix,
and every black-box adaptation are in
[`docs/05-e2e-test-plan.md`](./docs/05-e2e-test-plan.md).

---

## Project structure

```
.
├── cmd/server/            # entry point: wiring, slog, graceful shutdown, -health
├── internal/
│   ├── domain/            # entities + business rules (no I/O)
│   ├── store/             # Store interface + in-memory implementation
│   ├── service/           # application orchestration
│   └── handler/           # REST + WebSocket handlers, conn manager, metrics
├── pkg/id/                # ID generation
├── features/              # 9 godog .feature files (the 47 scenarios, executable)
├── tests/e2e/             # black-box e2e harness (build-tagged `e2e`, no internal imports)
├── scripts/e2e.sh         # boot server → wait /health → run godog → tear down
├── deploy/                # Prometheus + Grafana provisioning
├── .github/workflows/     # e2e CI (blocking gate + advisory perf job)
├── docs/
│   ├── 01..05-*.md        # PRD, test cases, architecture, plan, e2e test plan
│   ├── backend-implementation/  # how the backend works (deep dive)
│   └── postman/           # API contract + Postman collection
├── Dockerfile · docker-compose.yml · Makefile · .env.example
└── test.md                # original challenge brief
```

---

## API at a glance

| Transport | Endpoint | Purpose |
|-----------|----------|---------|
| HTTP | `POST /api/sessions` | Create a quiz session |
| HTTP | `GET /api/health` | Liveness + active-session count |
| HTTP | `GET /metrics` | Prometheus metrics |
| WebSocket | `GET /ws?quiz_id=&user_id=&name=` | Join + real-time messaging |

Full request/response contract — including every WebSocket message and error —
lives in [`docs/postman/`](./docs/postman/) (importable Postman collection) and is
explained in the backend implementation guide.

---

## AI collaboration

This project was built in collaboration with Claude (Anthropic). The planning
docs, test cases, architecture, the TDD implementation, and this documentation
were produced with AI assistance and verified via the test suite and linters.
Each doc notes its specific AI usage in an "AI Collaboration" section.
