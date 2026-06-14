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
| godog BDD feature files | — | ⏳ optional (cases covered by Go tests) |

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
├── deploy/                # Prometheus + Grafana provisioning
├── docs/
│   ├── 01..04-*.md        # PRD, test cases, architecture, plan
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
