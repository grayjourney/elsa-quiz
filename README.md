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
| WebSocket/HTTP layer, Docker, Makefile | — | ⏳ next increment |

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
| Real-time transport | WebSocket (planned: `gorilla/websocket`) |
| Persistence | In-memory behind a `Store` interface (swap for Redis/PG later) |
| Architecture | Clean/layered: `domain` → `store` → `service` → `handler` |
| Testing | `testing` + table-driven tests, `-race`, `godog` (BDD, planned) |
| Tooling | `golangci-lint`, Makefile + Docker (planned) |

---

## Quick start (current state)

The runnable server (WebSocket/HTTP) is the next increment. Today you can build
and exercise the full business core via the test suite:

```bash
go test ./...                                  # all packages green
go test -race ./...                            # no data races
go vet ./internal/... ./pkg/...                # clean
golangci-lint run ./internal/... ./pkg/...     # 0 issues
go test -cover ./internal/... ./pkg/...        # coverage report
```

Coverage: domain **93.7%**, service **93.9%**, store **100%**.

---

## Project structure

```
.
├── cmd/server/            # entry point (next increment)
├── internal/
│   ├── domain/            # entities + business rules (no I/O)
│   ├── store/             # Store interface + in-memory implementation
│   ├── service/           # application orchestration
│   ├── handler/           # WebSocket/HTTP (next increment)
│   └── server/            # wiring (next increment)
├── pkg/id/                # ID generation
├── docs/
│   ├── 01..04-*.md        # PRD, test cases, architecture, plan
│   ├── backend-implementation/  # how the backend works (deep dive)
│   └── postman/           # API contract + Postman collection
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
