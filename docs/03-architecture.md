# System Design Document — Real-Time Vocabulary Quiz
# Architecture & Component Design

**Version:** 1.0  
**Date:** 2026-06-13  
**Status:** Proposed  
**Derived From:** [Product Requirements](./01-product-requirements.md), [Test Cases](./02-test-cases.md)

---

## 1. Architecture Overview

### 1.1 Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          REAL-TIME QUIZ SYSTEM                             │
│                                                                             │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐                               │
│  │ Client A │   │ Client B │   │ Client N │    (Web/Mobile Clients)        │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘                               │
│       │              │              │                                       │
│       └──────────────┼──────────────┘                                       │
│                      │  WebSocket (ws://)                                    │
│                      ▼                                                      │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                      API / WebSocket Gateway                          │  │
│  │  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────────────┐   │  │
│  │  │  HTTP REST   │  │ WebSocket Server│  │  Connection Manager     │   │  │
│  │  │ (health,     │  │ (gorilla/ws)    │  │  (track connections,    │   │  │
│  │  │  session     │  │                 │  │   handle reconnect)     │   │  │
│  │  │  create)     │  │                 │  │                         │   │  │
│  │  └─────────────┘  └─────────────────┘  └─────────────────────────┘   │  │
│  └──────────────────────────┬────────────────────────────────────────────┘  │
│                             │                                               │
│  ┌──────────────────────────▼────────────────────────────────────────────┐  │
│  │                       CORE DOMAIN LAYER                               │  │
│  │                                                                       │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────────┐ │  │
│  │  │ Quiz Session  │  │   Scoring    │  │     Leaderboard             │ │  │
│  │  │  Manager      │  │   Engine     │  │     Engine                  │ │  │
│  │  │              │  │              │  │                             │ │  │
│  │  │ - Create     │  │ - Validate   │  │ - Rank participants        │ │  │
│  │  │ - Join       │  │ - Calculate  │  │ - Handle tie-breaking      │ │  │
│  │  │ - Track      │  │ - Broadcast  │  │ - Broadcast updates        │ │  │
│  │  │   state      │  │   scores     │  │                             │ │  │
│  │  └──────────────┘  └──────────────┘  └─────────────────────────────┘ │  │
│  └──────────────────────────┬────────────────────────────────────────────┘  │
│                             │                                               │
│  ┌──────────────────────────▼────────────────────────────────────────────┐  │
│  │                    INFRASTRUCTURE LAYER                               │  │
│  │                                                                       │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────────┐ │  │
│  │  │  In-Memory   │  │  Structured  │  │    Metrics / Health         │ │  │
│  │  │  Store       │  │  Logger      │  │    (Prometheus, pprof)      │ │  │
│  │  │  (sessions,  │  │  (slog)      │  │                             │ │  │
│  │  │   scores)    │  │              │  │                             │ │  │
│  │  └──────────────┘  └──────────────┘  └─────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                    EXTERNAL (MOCKED IN MVP)                           │  │
│  │                                                                       │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────────┐ │  │
│  │  │  Database    │  │  Auth        │  │    Message Broker           │ │  │
│  │  │  (Redis/PG)  │  │  Service     │  │    (for horizontal scale)  │ │  │
│  │  │  [MOCKED]    │  │  [MOCKED]    │  │    [MOCKED]                │ │  │
│  │  └──────────────┘  └──────────────┘  └─────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Project Classification

Using the architecture skill's context discovery matrix:

| Dimension | Value | Rationale |
|-----------|-------|-----------|
| **Scale** | MVP → SaaS ready (100-1K users) | Challenge scope, but designed for growth |
| **Team** | Solo developer | Coding challenge |
| **Timeline** | Fast (days) | Challenge deliverable |
| **Architecture** | Modular Monolith | Simple start, extractable later |
| **Patterns** | Selective | Event-driven for real-time, simple data access |

---

## 2. Component Descriptions

### 2.1 API / WebSocket Gateway

| Aspect | Detail |
|--------|--------|
| **Role** | Entry point for all client communication |
| **Responsibilities** | Accept HTTP requests (create session, health check), upgrade to WebSocket for real-time communication, manage connection lifecycle |
| **Technology** | Go `net/http` + `gorilla/websocket` |
| **Key Interfaces** | `POST /api/sessions` (create), `GET /api/health` (health check), `WS /ws?quiz_id=X&user_id=Y` (real-time) |

### 2.2 Connection Manager

| Aspect | Detail |
|--------|--------|
| **Role** | Track all active WebSocket connections per session |
| **Responsibilities** | Register/unregister connections, broadcast messages to all session participants, handle disconnection/reconnection |
| **Concurrency** | Thread-safe with `sync.RWMutex` for connection maps |

### 2.3 Quiz Session Manager

| Aspect | Detail |
|--------|--------|
| **Role** | Core domain entity managing quiz session state |
| **Responsibilities** | Create sessions with unique IDs, add/remove participants, manage session lifecycle (waiting → active → completed), validate operations against session state |
| **Concurrency** | Per-session mutex to prevent race conditions |

### 2.4 Scoring Engine

| Aspect | Detail |
|--------|--------|
| **Role** | Calculate and validate scores |
| **Responsibilities** | Validate answers against correct answers, calculate points, prevent duplicate submissions, ensure scoring accuracy and consistency |
| **Design** | Pure function with no side effects — easy to test |

### 2.5 Leaderboard Engine

| Aspect | Detail |
|--------|--------|
| **Role** | Maintain and broadcast real-time rankings |
| **Responsibilities** | Calculate rankings from scores, handle tie-breaking (earlier submission wins), generate leaderboard snapshots for broadcast |
| **Design** | Stateless calculation — derived from each participant's current `Score` and `LastScoredAt` timestamp. Ordering: `Score DESC, LastScoredAt ASC` so equal scores break deterministically in favour of whoever reached the score first. The Scoring Engine must stamp `LastScoredAt` whenever it mutates a score. |

### 2.6 In-Memory Store

| Aspect | Detail |
|--------|--------|
| **Role** | Data persistence for MVP |
| **Responsibilities** | Store sessions, participants, scores, answers |
| **Design** | Interface-based so it can be swapped for Redis/PostgreSQL later |
| **Trade-off** | Data lost on server restart (acceptable for MVP, addressed in future with persistence layer) |

### 2.7 Observability Layer

| Aspect | Detail |
|--------|--------|
| **Role** | Monitoring and diagnostics |
| **Components** | Structured logging (`slog`), Prometheus metrics, health check endpoint, pprof profiling |
| **Key Metrics** | Active connections, messages/sec, score calculation latency, leaderboard calculation time, error rates |
| **Log viewing** | The server logs significant events (session created, player joined, answer scored, quiz ended, errors) as JSON to stdout. In the Docker stack, **Promtail** tails the quiz container and ships those logs to **Loki**, so they are queryable in **Grafana** (a "Server logs" panel on the overview dashboard, plus Explore with `{job="quiz"}`). Without Docker, the same logs stream in the terminal via `make logs`. |

### 2.8 Quiz Scheduler (Timed Advancement)

| Aspect | Detail |
|--------|--------|
| **Role** | Drive automatic question advancement for the `timed` end policy |
| **Responsibilities** | Hold one per-session `time.AfterFunc` timer; on expiry advance the current question; advance early once all *connected* participants have answered; auto-complete after the last question; cancel timers on quiz end / shutdown |
| **Design** | Lives in the handler layer (the domain aggregate stays pure). All three advance triggers (timer fire, all-answered, host action) funnel through the aggregate's guarded `AdvanceIfCurrent(questionID)`, which advances only if that question is still current — making concurrent triggers race-free (no double-advance) |
| **Manual policy** | No timers; the host drives advancement |

---

## 3. Data Flow

### 3.1 User Joins a Quiz

```
Client                    Gateway              SessionManager         Store
  │                         │                       │                   │
  │ WS Connect              │                       │                   │
  │ quiz_id=ABC             │                       │                   │
  │ user_id=Alice           │                       │                   │
  │ ───────────────────────>│                       │                   │
  │                         │ Validate session      │                   │
  │                         │ ─────────────────────>│                   │
  │                         │                       │ Lookup session    │
  │                         │                       │ ─────────────────>│
  │                         │                       │ <─────────────────│
  │                         │                       │                   │
  │                         │                       │ Add participant   │
  │                         │                       │ ─────────────────>│
  │                         │ <─────────────────────│                   │
  │                         │                       │                   │
  │                         │ Register connection   │                   │
  │                         │ (ConnectionManager)   │                   │
  │                         │                       │                   │
  │ Join confirmed          │                       │                   │
  │ <───────────────────────│                       │                   │
  │                         │                       │                   │
  │                         │ Broadcast: user_joined│                   │
  │                         │ ──────> All clients   │                   │
```

### 3.2 Answer Submission → Score Update → Leaderboard

```
Client(Alice)     Gateway       ScoringEngine    LeaderboardEngine    All Clients
  │                 │                │                  │                  │
  │ Submit answer   │                │                  │                  │
  │ Q1: "went"      │                │                  │                  │
  │ ───────────────>│                │                  │                  │
  │                 │ Validate &     │                  │                  │
  │                 │ Score answer   │                  │                  │
  │                 │ ──────────────>│                  │                  │
  │                 │                │                  │                  │
  │                 │ Score result:  │                  │                  │
  │                 │ +10 points     │                  │                  │
  │                 │ <──────────────│                  │                  │
  │                 │                │                  │                  │
  │                 │ Recalculate    │                  │                  │
  │                 │ leaderboard    │                  │                  │
  │                 │ ──────────────────────────────────>│                  │
  │                 │                │                  │                  │
  │                 │ Leaderboard    │                  │                  │
  │                 │ snapshot       │                  │                  │
  │                 │ <──────────────────────────────────│                  │
  │                 │                │                  │                  │
  │                 │ Broadcast:     │                  │                  │
  │                 │ score_update + │                  │                  │
  │                 │ leaderboard    │                  │                  │
  │                 │ ─────────────────────────────────────────────────────>│
  │                 │                │                  │                  │
```

---

## 4. Technologies and Tools

### 4.1 Chosen Stack

| Component | Technology | Justification |
|-----------|------------|---------------|
| **Language** | Go 1.21+ | High concurrency (goroutines), low latency, strong stdlib, excellent for real-time systems |
| **WebSocket** | `gorilla/websocket` | Industry-standard Go WebSocket library, battle-tested |
| **HTTP Router** | `net/http` (stdlib) | No external dependency needed for simple routing |
| **Logging** | `log/slog` (stdlib) | Structured logging built into Go 1.21+, zero dependencies |
| **Metrics** | `prometheus/client_golang` | Industry standard, integrates with Grafana |
| **Log viewing** | Grafana Loki + Promtail | Make `slog` output viewable in Grafana (Docker stack); Promtail tails the quiz container and pushes to Loki |
| **Testing** | `testing` (stdlib) + `testify` | Table-driven tests, assertions, mocking |
| **BDD Tests** | `godog` (Cucumber for Go) | Maps directly to Gherkin test cases |
| **ID Generation** | `google/uuid` | RFC 4122 UUIDs for session IDs |
| **Profiling** | `net/http/pprof` (stdlib) | Built-in CPU/memory profiling |
| **Build** | `Makefile` | Standard Go project automation |
| **Containerization** | Docker (multi-stage) + Docker Compose | Multi-stage build → minimal `distroless` runtime image; Compose for one-command local bring-up of the server + observability stack |
| **Local orchestration** | `docker compose` (with profiles) | Runs server, Prometheus, and Grafana together for a complete local demo |

> **Runtime base-image note:** the production stage uses **`gcr.io/distroless/static`** (not `scratch`). Distroless gives a near-`scratch` footprint and CA certs, while still allowing a container `HEALTHCHECK` driven by the app's own `/api/health` endpoint (the binary supports a `-health` self-check flag so no shell/curl is required inside the image). A bare `scratch` image cannot run the `HEALTHCHECK` examples that the observability requirements (NFR-5.1) imply.

### 4.3 Two Local Run Modes

The setup deliberately supports **two ways to run the project**, so a reviewer can pick speed of iteration vs. zero-toolchain convenience:

| Mode | Server runs as | Infra (Prometheus/Grafana) | Use when |
|------|----------------|----------------------------|----------|
| **A — Full Docker** | Container (`quiz`) | Containers | You only have Docker; want a one-command demo identical to CI |
| **B — Go native + Docker infra** | Host process (`go run` / `make run`) | Containers | You have Go installed and want fast rebuilds, debugger attach, hot edit-run cycles |

**Implementation requirement this imposes:** Prometheus must reach the server in *both* topologies. This is solved with **Docker Compose profiles** + a single scrape config that lists two targets:

```yaml
# deploy/prometheus/prometheus.yml
scrape_configs:
  - job_name: quiz
    static_configs:
      - targets:
          - quiz:8080                # Mode A: sibling container (service DNS)
          - host.docker.internal:8080 # Mode B: server on the host
```

- Compose declares `host.docker.internal` via `extra_hosts: ["host.docker.internal:host-gateway"]` on the Prometheus service so Mode B works on Linux as well as macOS/Windows.
- In each mode one of the two targets is simply reported "down" by Prometheus — harmless, and it makes the active topology obvious on the Targets page.
- **Compose profiles:** the `quiz` service is gated behind the `full` profile. `docker compose --profile full up` → Mode A (everything). `docker compose up` (no profile) → infra only, for Mode B.

### 4.2 Architecture Decision Records

#### ADR-001: Go over Node.js/Python for Server Component

| Aspect | Detail |
|--------|--------|
| **Context** | Need a real-time server handling WebSocket connections with high concurrency |
| **Decision** | Go |
| **Rationale** | Goroutines provide lightweight concurrency (vs. Node.js event loop); static typing catches bugs at compile time; excellent performance for I/O-bound workloads; single binary deployment |
| **Trade-off** | Less ecosystem for web frameworks (acceptable — we need minimal HTTP) |
| **Revisit trigger** | If team has no Go experience |

#### ADR-002: In-Memory Store over Database for MVP

| Aspect | Detail |
|--------|--------|
| **Context** | Need to store sessions, participants, and scores |
| **Decision** | In-memory maps behind an interface |
| **Rationale** | Simplest approach for MVP; no external dependency; fastest possible reads/writes; interface allows swapping to Redis/PostgreSQL later |
| **Trade-off** | Data lost on restart (acceptable for quiz sessions which are ephemeral) |
| **Revisit trigger** | Need persistence across restarts, or horizontal scaling requires shared state |

#### ADR-003: Event-Driven Architecture for Real-Time Updates

| Aspect | Detail |
|--------|--------|
| **Context** | Real-time score and leaderboard updates required |
| **Decision** | WebSocket with server-push broadcasting |
| **Rationale** | Low latency (< 500ms requirement); bidirectional communication; efficient for frequent small updates; widely supported by clients |
| **Trade-off** | More complex than REST polling; requires connection state management |
| **Revisit trigger** | If eventual consistency is too slow or if we need pub/sub across multiple server instances |

#### ADR-004: Modular Monolith over Microservices

| Aspect | Detail |
|--------|--------|
| **Context** | Solo developer, MVP scope, need clear component boundaries |
| **Decision** | Modular monolith with clean package boundaries |
| **Rationale** | Single deployment unit; no network overhead between components; can extract to microservices later if needed; matches team size |
| **Trade-off** | All components scale together (acceptable at MVP scale) |
| **Revisit trigger** | Team > 5 developers, or components need independent scaling |

---

## 5. Project Structure

```
elsa-test/
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point
├── internal/
│   ├── domain/                     # Core domain models (no dependencies)
│   │   ├── quiz.go                 # QuizSession, Question entities
│   │   ├── participant.go          # Participant entity
│   │   ├── score.go                # Score value object
│   │   ├── leaderboard.go          # LeaderboardEntry, ranking logic
│   │   └── errors.go               # Domain-specific errors
│   ├── service/                    # Application/business logic
│   │   ├── session_service.go      # Quiz session management
│   │   ├── scoring_service.go      # Answer validation & scoring
│   │   └── leaderboard_service.go  # Leaderboard calculation
│   ├── handler/                    # HTTP/WebSocket handlers + server wiring
│   │   ├── ws_handler.go           # WebSocket upgrade & message routing
│   │   ├── api.go                  # REST endpoints (create/join/start/advance/end/health) + mux
│   │   ├── static.go               # serves the embedded web client at GET /
│   │   ├── connection_manager.go   # per-session connection registry + broadcast
│   │   ├── scheduler.go            # timed-policy per-session timer
│   │   ├── metrics.go              # Prometheus metrics
│   │   └── message.go              # WebSocket message types
│   └── store/                      # Data access layer (interface + impl)
│       ├── store.go                # Store interface
│       └── memory_store.go         # In-memory implementation
├── pkg/id/generator.go             # ID generation (crypto/rand)
├── web/                            # embedded single-file web client (served at GET /)
├── tests/e2e/                      # black-box godog suite (real WS+HTTP, build-tagged `e2e`)
├── scripts/e2e.sh                  # boot server → run godog → tear down
├── docs/                           # 01..06 + backend-implementation/ + postman/
├── features/                       # godog .feature files (9 files, the 47 scenarios)
│   ├── quiz_session_management.feature   # + quiz_participation, score_updates,
│   ├── session_lifecycle.feature         #   leaderboard, connection_reliability,
│   └── operations_observability.feature  #   input_validation, performance_under_load
├── deploy/                         # observability config
│   ├── prometheus/prometheus.yml   # scrape config for the quiz server
│   ├── promtail/promtail.yml       # ships the quiz container's logs to Loki
│   └── grafana/
│       ├── provisioning/           # datasources (Prometheus + Loki) + dashboard provider
│       └── dashboards/             # quiz-overview dashboard (metrics + logs panel)
├── .github/workflows/e2e.yml       # CI: blocking e2e gate + advisory perf job
├── Makefile                        # Build, test, run, docker commands
├── Dockerfile                      # Multi-stage build → distroless runtime
├── docker-compose.yml              # server + prometheus + grafana (local stack)
├── .dockerignore                   # Lean build context
├── .env.example                    # Documented configuration knobs
├── go.mod
├── go.sum
└── README.md                       # Setup, run, test, API & demo guide
```

---

## 6. Scalability Design

### 6.1 Current (Single Instance)

- In-memory store handles all state
- Single Go process with goroutines for concurrency
- Sufficient for 100+ concurrent users per session

### 6.2 Future (Horizontal Scale)

```
                    Load Balancer (sticky sessions)
                    ┌───────┬───────┐
                    ▼       ▼       ▼
              ┌─────────┐ ┌─────────┐ ┌─────────┐
              │Server 1 │ │Server 2 │ │Server 3 │
              └────┬────┘ └────┬────┘ └────┬────┘
                   │           │           │
                   └───────────┼───────────┘
                               ▼
                    ┌─────────────────┐
                    │  Redis Pub/Sub  │   (shared state + message broker)
                    └────────┬────────┘
                             ▼
                    ┌─────────────────┐
                    │   PostgreSQL    │   (persistent storage)
                    └─────────────────┘
```

### 6.3 Scalability Decisions

| Concern | MVP Approach | Scale Approach |
|---------|-------------|----------------|
| State | In-memory maps | Redis |
| Broadcast | Direct goroutine fan-out | Redis Pub/Sub |
| Persistence | None (ephemeral) | PostgreSQL |
| Load balancing | N/A (single instance) | Sticky sessions by quiz ID |
| Session affinity | N/A | Quiz ID → server mapping |

---

## 7. Monitoring & Observability

| Signal | Tool | What |
|--------|------|------|
| **Logs** | `slog` → Promtail → Loki → Grafana | All significant events (session created, join, answer/score, quiz ended, error) as structured JSON; viewable in Grafana (Loki) or `make logs` |
| **Metrics** | Prometheus | `quiz_active_sessions`, `quiz_connected_users`, `quiz_message_latency_ms`, `quiz_score_calculations_total`, `quiz_errors_total` |
| **Health** | `/api/health` endpoint | Server liveness, active sessions count |
| **Profiling** | `pprof` | CPU/memory profiling on demand |
| **Tracing** | OpenTelemetry (future) | Distributed tracing for multi-service setup |

---

## 8. Trade-off Summary

| Decision | Benefit | Cost | Acceptable Because |
|----------|---------|------|-------------------|
| In-memory over DB | Zero latency, no deps | Data loss on crash | Quiz sessions are ephemeral |
| Modular monolith | Simple deployment | Can't scale independently | Solo developer, MVP scope |
| gorilla/websocket | Battle-tested | External dependency | One well-maintained library |
| No auth | Simpler implementation | No security | Mocked per requirements |
| No persistence | Faster development | No history | Out of scope for MVP |

---

## 9. AI Collaboration in Design

- **Tool**: Gemini / Claude (Architecture skill)
- **Tasks**: Component decomposition, data flow design, ADR authoring, pattern selection
- **Verification**: All architectural decisions validated against the architecture skill's pattern selection decision tree; simplicity principle applied throughout; each ADR includes revisit triggers for future reassessment
