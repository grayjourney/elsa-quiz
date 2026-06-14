# Implementation Plan — Real-Time Vocabulary Quiz
# TDD-Driven, Go Implementation

**Version:** 1.1  
**Date:** 2026-06-14  
**Status:** Core complete (Steps 1–18, 20 done) — see Progress below  
**Derived From:** [PRD](./01-product-requirements.md) · [Test Cases](./02-test-cases.md) · [Architecture](./03-architecture.md)

---

## Progress (2026-06-14)

| Phase / Steps | Status |
|---------------|--------|
| 1 — Foundation (module, structure, **Docker**, **Makefile**), 2 ID gen | ✅ done |
| 2 — Domain (Steps 3–8) | ✅ done |
| 3 — Store (Step 9) | ✅ done |
| 4 — Services (Steps 10–12) | ✅ done |
| 5 — Handlers: WS protocol/conn-manager/WS handler/HTTP (Steps 13–16) | ✅ done |
| 6 — Server wiring, slog, graceful shutdown, `/metrics`, pprof (Step 17) | ✅ done |
| 7 — Integration tests (Step 18: full flow, reconnect, concurrency over WS) | ✅ done |
| 8 — Prometheus instrumentation (Step 20), validation (21), README (22) | ✅ done |
| 7 — `godog` BDD (Step 19) | ⏳ optional — covered by Go integration/e2e tests |

> Deviations from the original step text: ID generator uses stdlib `crypto/rand`
> (not `google/uuid`); HTTP wiring lives in `cmd/server/main.go` (no separate
> `internal/server` package); `manual`/`timed` end policies were added (FR-5).

---

## Approach

Build the real-time quiz server component in Go using strict Test-Driven Development (RED → GREEN → REFACTOR). Start with pure domain models and scoring logic (no I/O), layer on the WebSocket transport, then wire everything together. Every production function starts with a failing test. Mocks are used only at I/O boundaries (WebSocket connections). BDD feature files validate end-to-end acceptance criteria.

---

## Scope

### In
- Go server handling WebSocket connections, scoring, and leaderboard
- Domain models: QuizSession, Participant, Question, Answer, LeaderboardEntry
- In-memory store behind an interface
- WebSocket message protocol (JSON)
- Structured logging, health check endpoint, Prometheus metrics
- Unit tests (≥ 80% coverage), integration tests, BDD feature tests
- Docker multi-stage build
- Makefile for build/test/run automation

### Out
- Client application (web/mobile)
- Database persistence (mocked via interface)
- Authentication service (mocked — user ID passed at connection)
- Quiz content authoring
- Horizontal scaling infrastructure (Redis pub/sub) — designed for, not implemented
- CI/CD pipeline configuration

---

## Action Items

Each step follows the TDD cycle: **write failing test → write minimal code → refactor**. Steps are ordered by dependency (domain → service → handler → wiring).

### Phase 1: Project Foundation

- [ ] **Step 1: Initialize Go module and project structure**
  - Run `go mod init github.com/gray/elsa-quiz`
  - Create directory structure: `cmd/server/`, `internal/{domain,service,handler,store,server}/`, `pkg/id/`, `features/`, `deploy/{prometheus,grafana}/`
  - **No TDD needed** — infrastructure/config setup (TDD skill explicitly exempts config & generated scaffolding)

- [ ] **Step 1a: Author the local Docker setup (docker-expert review)**
  - **`Dockerfile` (multi-stage):**
    - `builder` stage on `golang:1.22-alpine`: leverage layer caching — `COPY go.mod go.sum` then `RUN go mod download` *before* copying source, so deps re-cache only when they change.
    - `RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /quiz ./cmd/server`
    - **Final stage on `gcr.io/distroless/static:nonroot`** (not `scratch`): runs as non-root UID 65532, includes CA certs, near-scratch size.
    - `EXPOSE 8080`; `USER nonroot:nonroot`; `ENTRYPOINT ["/quiz"]`
    - `HEALTHCHECK` invokes the binary's own `-health` self-check flag (hits `/api/health` locally) — no shell/curl needed in a distroless image.
  - **`.dockerignore`:** exclude `.git`, `docs/`, `*.md`, `bin/`, test artifacts, `*_test.go` → lean build context, faster builds.
  - **`docker-compose.yml`:** three services on a user-defined bridge network, designed to support **both run modes** (see Architecture §4.3) via Compose profiles —
    - `quiz` (build: `.`) — **gated behind the `full` profile** so it only starts in Mode A. Ports `8080:8080`, env from `.env`, `healthcheck` on `/api/health`, `restart: unless-stopped`, resource limits.
    - `prometheus` (`prom/prometheus`): mounts `deploy/prometheus/prometheus.yml`; scrapes **both** `quiz:8080` (Mode A) and `host.docker.internal:8080` (Mode B); declares `extra_hosts: ["host.docker.internal:host-gateway"]` so host scraping works on Linux too.
    - `grafana` (`grafana/grafana`): port `3000:3000`, pre-provisioned datasource + dashboard from `deploy/grafana/`.
  - **Two ways to bring it up:**
    - **Mode A (full Docker):** `docker compose --profile full up --build` → server + Prometheus + Grafana, all in containers.
    - **Mode B (native + infra):** `docker compose up` (no profile) → only Prometheus + Grafana; the server is then started on the host with `make run`.
  - **`.env.example`:** documented knobs — `PORT`, `LOG_LEVEL`, `BASE_POINTS`, `SHUTDOWN_TIMEOUT`.
  - **Validation:** `docker compose config` parses clean; both `--profile full up` and the no-profile (infra-only) bring-up are healthy.
  - **No TDD needed** — infrastructure/config.

- [ ] **Step 1b: Author the `Makefile` (common project commands)**
  - **Build/run (native — Mode B):** `build`, `run` (local `go run ./cmd/server`), `tidy`
  - **Test:** `test`, `test-race` (`go test -race ./...`), `test-cover` (writes `coverage.out` + prints %), `bdd` (`go test ./features/...`)
  - **Quality:** `lint` (`golangci-lint run`), `fmt` (`gofmt -w` + `goimports`)
  - **Full Docker stack (Mode A):** `docker-build`, `up` (`docker compose --profile full up --build -d`), `down` (`docker compose --profile full down`), `logs`, `ps`
  - **Infra only (Mode B helper):** `infra-up` (`docker compose up -d` → Prometheus + Grafana only), `infra-down` (`docker compose down`) — pair with `make run` for native dev with live observability
  - **Meta:** `help` (self-documenting default target that lists all commands), `.PHONY` on every target
  - **No TDD needed** — infrastructure/config.

- [ ] **Step 2: Implement ID generator (TDD)**
  - 🔴 RED: Write test `TestGenerateID_ReturnsUniqueIDs` — generate 1000 IDs, assert all unique
  - 🔴 RED: Write test `TestGenerateID_ReturnsNonEmptyString`
  - 🟢 GREEN: Implement `pkg/id/generator.go` using `google/uuid`
  - 🔵 REFACTOR: None expected (simple utility)
  - **File**: `pkg/id/generator.go`, `pkg/id/generator_test.go`

### Phase 2: Domain Models (Pure Logic, No I/O)

- [ ] **Step 3: Implement domain errors (TDD)**
  - 🔴 RED: Write test `TestErrSessionNotFound_ImplementsError` — assert error message
  - 🟢 GREEN: Define sentinel errors in `internal/domain/errors.go`:
    - `ErrSessionNotFound`, `ErrSessionCompleted`, `ErrQuizIDRequired`
    - `ErrQuestionNotFound`, `ErrDuplicateAnswer`, `ErrAnswerEmpty`, `ErrInvalidOption`
    - `ErrQuizEnded`
  - **File**: `internal/domain/errors.go`, `internal/domain/errors_test.go`

- [ ] **Step 4: Implement Question entity (TDD)**
  - 🔴 RED: Write table-driven test `TestQuestion_Validate` — valid question passes, missing fields fail
  - 🔴 RED: Write test `TestQuestion_IsCorrectAnswer` — correct/incorrect answers
  - 🟢 GREEN: Implement `internal/domain/quiz.go` — `Question` struct with `Validate()` and `IsCorrectAnswer(answer string) bool`
  - 🔵 REFACTOR: Extract validation logic if needed
  - **File**: `internal/domain/quiz.go`, `internal/domain/quiz_test.go`

- [ ] **Step 5: Implement Participant entity (TDD)**
  - 🔴 RED: Write test `TestNewParticipant_InitializesWithZeroScore`
  - 🔴 RED: Write test `TestParticipant_AddScore_IncreasesScore`
  - 🔴 RED: Write test `TestParticipant_AddScore_StampsLastScoredAt` — `LastScoredAt` advances on each scoring (needed for deterministic tie-breaking, FR-4.6)
  - 🔴 RED: Write test `TestParticipant_HasAnswered_ReturnsTrueForAnsweredQuestion`
  - 🔴 RED: Write test `TestParticipant_RecordAnswer_PreventsResubmission`
  - 🟢 GREEN: Implement `internal/domain/participant.go` — `Participant` struct
  - 🔵 REFACTOR: Ensure thread safety with mutex if needed
  - **File**: `internal/domain/participant.go`, `internal/domain/participant_test.go`

- [ ] **Step 6: Implement Score value object (TDD)**
  - 🔴 RED: Write test `TestCalculateScore_CorrectAnswer_ReturnsBasePoints`
  - 🔴 RED: Write test `TestCalculateScore_IncorrectAnswer_ReturnsZero`
  - 🟢 GREEN: Implement `internal/domain/score.go` — pure function `CalculateScore(isCorrect bool, basePoints int) int`
  - 🔵 REFACTOR: None expected (pure function)
  - **File**: `internal/domain/score.go`, `internal/domain/score_test.go`

- [ ] **Step 7: Implement Leaderboard logic (TDD)**
  - 🔴 RED: Write test `TestLeaderboard_RanksByScoreDescending`
  - 🔴 RED: Write test `TestLeaderboard_TieBreaking_EarlierScoreWins`
  - 🔴 RED: Write test `TestLeaderboard_EmptyParticipants_ReturnsEmptySlice`
  - 🔴 RED: Write test `TestLeaderboard_SingleParticipant_RankOne`
  - 🟢 GREEN: Implement `internal/domain/leaderboard.go` — `CalculateLeaderboard(participants []Participant) []LeaderboardEntry`
  - 🔵 REFACTOR: Optimize sort if needed
  - **File**: `internal/domain/leaderboard.go`, `internal/domain/leaderboard_test.go`

- [ ] **Step 8: Implement QuizSession entity (TDD)**
  - 🔴 RED: Write test `TestNewQuizSession_StatusIsWaiting`
  - 🔴 RED: Write test `TestQuizSession_AddParticipant_AddsToList`
  - 🔴 RED: Write test `TestQuizSession_AddParticipant_ToCompletedSession_ReturnsError`
  - 🔴 RED: Write test `TestQuizSession_Start_ChangesStatusToActive`
  - 🔴 RED: Write test `TestQuizSession_Complete_ChangesStatusToCompleted`
  - 🔴 RED: Write test `TestQuizSession_SubmitAnswer_ToCompletedSession_ReturnsError`
  - 🟢 GREEN: Implement `internal/domain/quiz.go` — `QuizSession` struct with lifecycle methods
  - 🔵 REFACTOR: Extract state machine pattern if complex
  - **File**: `internal/domain/quiz.go`, `internal/domain/quiz_test.go`

### Phase 3: Store Layer (Interface + In-Memory)

- [ ] **Step 9: Define Store interface and implement in-memory store (TDD)**
  - 🔴 RED: Write test `TestMemoryStore_CreateSession_ReturnsSession`
  - 🔴 RED: Write test `TestMemoryStore_GetSession_ReturnsErrNotFound`
  - 🔴 RED: Write test `TestMemoryStore_GetSession_ExistingSession_ReturnsSession`
  - 🔴 RED: Write test `TestMemoryStore_ConcurrentAccess_NoRaceConditions` (run with `-race`)
  - 🟢 GREEN: Define `Store` interface in `internal/store/store.go`, implement `MemoryStore` in `internal/store/memory_store.go`
  - 🔵 REFACTOR: Ensure mutex granularity is correct (per-session vs global)
  - **File**: `internal/store/store.go`, `internal/store/memory_store.go`, `internal/store/memory_store_test.go`

### Phase 4: Service Layer (Business Logic Orchestration)

- [ ] **Step 10: Implement Session Service (TDD)**
  - 🔴 RED: Write test `TestSessionService_CreateSession_ReturnsNewSession`
  - 🔴 RED: Write test `TestSessionService_JoinSession_AddsParticipant`
  - 🔴 RED: Write test `TestSessionService_JoinSession_NonExistent_ReturnsError`
  - 🔴 RED: Write test `TestSessionService_JoinSession_CompletedSession_ReturnsError`
  - 🔴 RED: Write test `TestSessionService_JoinSession_EmptyQuizID_ReturnsError`
  - 🟢 GREEN: Implement `internal/service/session_service.go`
  - **File**: `internal/service/session_service.go`, `internal/service/session_service_test.go`

- [ ] **Step 11: Implement Scoring Service (TDD)**
  - 🔴 RED: Write test `TestScoringService_SubmitAnswer_CorrectAnswer_IncreasesScore`
  - 🔴 RED: Write test `TestScoringService_SubmitAnswer_IncorrectAnswer_NoScoreChange`
  - 🔴 RED: Write test `TestScoringService_SubmitAnswer_DuplicateAnswer_ReturnsError`
  - 🔴 RED: Write test `TestScoringService_SubmitAnswer_QuizCompleted_ReturnsError`
  - 🔴 RED: Write test `TestScoringService_SubmitAnswer_InvalidQuestion_ReturnsError`
  - 🟢 GREEN: Implement `internal/service/scoring_service.go`
  - 🔵 REFACTOR: Ensure scoring + leaderboard calculation is atomic
  - **File**: `internal/service/scoring_service.go`, `internal/service/scoring_service_test.go`

- [ ] **Step 12: Implement Leaderboard Service (TDD)**
  - 🔴 RED: Write test `TestLeaderboardService_GetLeaderboard_ReturnsRankedParticipants`
  - 🔴 RED: Write test `TestLeaderboardService_GetLeaderboard_EmptySession_ReturnsEmptySlice`
  - 🟢 GREEN: Implement `internal/service/leaderboard_service.go`
  - **File**: `internal/service/leaderboard_service.go`, `internal/service/leaderboard_service_test.go`

### Phase 5: WebSocket Handler (I/O Layer)

- [ ] **Step 13: Define WebSocket message protocol (TDD)**
  - 🔴 RED: Write test `TestMessage_MarshalJSON_CorrectFormat` — test all message types
  - 🔴 RED: Write test `TestMessage_UnmarshalJSON_ValidPayload`
  - 🔴 RED: Write test `TestMessage_UnmarshalJSON_InvalidPayload_ReturnsError`
  - 🟢 GREEN: Implement `internal/handler/message.go` — message types:
    - Client → Server: `join`, `submit_answer`
    - Server → Client: `join_confirmed`, `user_joined`, `question`, `score_update`, `leaderboard_update`, `error`, `quiz_ended`
  - **File**: `internal/handler/message.go`, `internal/handler/message_test.go`

- [ ] **Step 14: Implement Connection Manager (TDD)**
  - 🔴 RED: Write test `TestConnectionManager_Register_AddsConnection`
  - 🔴 RED: Write test `TestConnectionManager_Unregister_RemovesConnection`
  - 🔴 RED: Write test `TestConnectionManager_Broadcast_SendsToAllSessionConnections`
  - 🔴 RED: Write test `TestConnectionManager_Broadcast_SkipsDisconnected`
  - 🔴 RED: Write test `TestConnectionManager_ConcurrentAccess_NoRace` (run with `-race`)
  - 🟢 GREEN: Implement `internal/handler/connection_manager.go`
  - **Note**: Use mock WebSocket connections for unit tests
  - **File**: `internal/handler/connection_manager.go`, `internal/handler/connection_manager_test.go`

- [ ] **Step 15: Implement WebSocket handler (TDD)**
  - 🔴 RED: Write test `TestWSHandler_HandleJoin_ReturnsConfirmation`
  - 🔴 RED: Write test `TestWSHandler_HandleSubmitAnswer_BroadcastsScoreUpdate`
  - 🔴 RED: Write test `TestWSHandler_HandleSubmitAnswer_BroadcastsLeaderboard`
  - 🔴 RED: Write test `TestWSHandler_HandleInvalidMessage_ReturnsError`
  - 🟢 GREEN: Implement `internal/handler/ws_handler.go`
  - **File**: `internal/handler/ws_handler.go`, `internal/handler/ws_handler_test.go`

- [ ] **Step 16: Implement HTTP handler (TDD)**
  - 🔴 RED: Write test `TestHTTPHandler_CreateSession_Returns201WithQuizID`
  - 🔴 RED: Write test `TestHTTPHandler_HealthCheck_Returns200`
  - 🔴 RED: Write test `TestHTTPHandler_HealthCheck_IncludesActiveSessionCount`
  - 🟢 GREEN: Implement `internal/handler/http_handler.go`
  - **File**: `internal/handler/http_handler.go`, `internal/handler/http_handler_test.go`

### Phase 6: Server Wiring & Entry Point

- [ ] **Step 17: Implement server setup and dependency injection**
  - Wire all components: store → services → handlers → HTTP server
  - Configure structured logging (`slog`)
  - Register Prometheus metrics endpoint (`/metrics`)
  - Register pprof endpoint (`/debug/pprof/`)
  - Implement graceful shutdown with `context` and `os.Signal`
  - **File**: `internal/server/server.go`, `cmd/server/main.go`

### Phase 7: Integration & BDD Tests

- [ ] **Step 18: Write integration tests (TDD)**
  - 🔴 RED: Write test `TestIntegration_FullQuizFlow` — create session → join → answer → verify leaderboard
  - 🔴 RED: Write test `TestIntegration_ConcurrentUsers` — 100 goroutines joining and answering
  - 🔴 RED: Write test `TestIntegration_DisconnectReconnect` — simulate connection drop
  - 🟢 GREEN: Fix any issues discovered during integration
  - **File**: `internal/server/integration_test.go`

- [ ] **Step 19: Write BDD feature tests with godog**
  - Map Gherkin scenarios from `docs/02-test-cases.md` to `features/*.feature` files
  - Implement step definitions
  - Run full BDD suite
  - **Files**: `features/session.feature`, `features/scoring.feature`, `features/leaderboard.feature`, `features/steps_test.go`

### Phase 8: Polish & Documentation

- [ ] **Step 20: Add Prometheus metrics instrumentation**
  - Instrument: active connections, message latency histogram, score calculations counter, error counter
  - **File**: `internal/server/metrics.go`

- [ ] **Step 21: Run full validation**
  - `make test` — all unit tests pass
  - `make test-cover` — verify ≥ 80% coverage
  - `make test-race` — no race conditions
  - `make lint` — zero warnings
  - `make build` — binary builds successfully
  - `make docker-build` — Docker image builds
  - **Mode A:** `make up` — full local stack (server + Prometheus + Grafana) comes up healthy; `/api/health` returns 200; `make down` tears it down
  - **Mode B:** `make infra-up && make run` — native server is scraped by the containerized Prometheus (`quiz` target UP at `:9090/targets`); `make infra-down` tears it down

- [ ] **Step 22: Write README.md (detailed init & run guide)**
  - **Prerequisites (per setup option):**
    - *Option A only needs:* Docker + Docker Compose, `make`.
    - *Option B also needs:* Go 1.22+ on the host. Include platform install hints (macOS `brew install go`; Linux via official tarball / `apt`; Windows via `choco install golang` or installer) and a `go version` verification step.
  - **First-time init (both options):** `cp .env.example .env` (review the documented knobs).
  - **Option A — Run everything in Docker (zero Go toolchain):**
    1. `make up`  → builds image, starts server + Prometheus + Grafana
    2. Verify: `curl localhost:8080/api/health` → `200 OK`; Grafana on `:3000`, Prometheus on `:9090`
    3. `make logs` to tail; `make down` to stop
  - **Option B — Run the Go server natively, infra in Docker (fast iteration):**
    1. `make infra-up`  → starts only Prometheus + Grafana (server slot left for the host)
    2. `make run`  → runs the server natively on `:8080` (supports a debugger / instant rebuilds)
    3. Verify: `curl localhost:8080/api/health`; Prometheus auto-scrapes the host via `host.docker.internal:8080` — confirm the `quiz` target is **UP** at `localhost:9090/targets`
    4. `make infra-down` to stop the infra when done
  - **Decision hint:** "Just want to see it run?" → Option A. "Want to read/modify the code and re-run quickly?" → Option B.
  - **Makefile command reference** — table of every target and what it does (mirror `make help`)
  - **API documentation:** REST endpoints (`POST /api/sessions`, `GET /api/health`, `GET /metrics`) + WebSocket message protocol (client→server and server→client message shapes with JSON examples)
  - **Demo walkthrough:** how to drive a full quiz flow (create → join via `wscat`/sample client → submit answers → watch leaderboard) for the video submission
  - **Observability:** how to open the Grafana dashboard and what each panel shows
  - **Troubleshooting:** port-in-use (`lsof -i :8080`), Mode-B `quiz` target shown DOWN in Prometheus (server not started / wrong port), `host.docker.internal` on Linux (covered by `extra_hosts`)
  - **Architecture summary** with links to `docs/01–03`
  - **AI Collaboration** section (required by the challenge) — pointer to where AI-assisted code is annotated

---

## Validation

| Check | Command | Target |
|-------|---------|--------|
| Unit tests pass | `make test` | ✅ All green |
| Test coverage | `make test-cover` | ≥ 80% |
| Race detector | `go test -race ./...` | No races |
| Linter | `golangci-lint run` | Zero warnings |
| Docker build | `docker build -t quiz .` | Success |
| Integration test | `go test ./internal/server/ -run Integration` | All pass |
| BDD tests | `go test ./features/` | All scenarios pass |

---

## Open Questions

1. **Bonus scoring for speed** — Should we implement speed-based bonus points in MVP, or defer to a later phase? (Current decision: defer — marked "Nice to Have" in PRD)
2. **Max participants per session** — Should there be a hard cap? (Current assumption: no hard cap in MVP, but test with 100)
3. **Quiz timer** — Should questions have a time limit? (Current assumption: no timer in MVP, but architecture supports adding it)

---

## TDD Workflow Reminder

For every step above:

```
1. 🔴 RED    — Write ONE failing test → run it → confirm it FAILS for the right reason
2. 🟢 GREEN  — Write MINIMAL code to make it pass → run it → confirm ALL tests pass
3. 🔵 REFACTOR — Clean up → run tests → confirm ALL tests still pass
4. REPEAT for next test
```

**Iron Law**: No production code without a failing test first. If you wrote code before the test, delete it and start over.

---

## AI Collaboration in Planning

- **Tools**: Gemini / Claude (concise-planning, golang-pro, test-driven-development, tdd-workflow skills)
- **Tasks**: Plan decomposition, TDD step ordering, Go-idiomatic project structure, test strategy
- **Verification**: Plan validated against TDD skill's verification checklist; all steps follow RED-GREEN-REFACTOR; Go patterns follow golang-pro best practices (table-driven tests, interface composition, goroutine safety)
