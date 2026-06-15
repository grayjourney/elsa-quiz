# End-to-End (E2E) Test Plan — Real-Time Vocabulary Quiz

**Version:** 1.1
**Date:** 2026-06-15
**Status:** ✅ Implemented — `make e2e` green (42/42 Tier-1), `make e2e-perf` advisory
**Derived From:** [PRD](./01-product-requirements.md) · [Test Cases](./02-test-cases.md) · [Architecture](./03-architecture.md) · [Implementation Plan](./04-implementation-plan.md)
**Harness:** `godog` v0.15.1 (Cucumber for Go) driving a **running** server over real WebSocket + HTTP
**Code:** [`features/`](../features) (9 `.feature` files) · [`tests/e2e/`](../tests/e2e) (harness) · [`scripts/e2e.sh`](../scripts/e2e.sh) · [`.github/workflows/e2e.yml`](../.github/workflows/e2e.yml)

---

## 1. Purpose & Scope

This document specifies the **end-to-end (E2E) test layer** for the Real-Time
Vocabulary Quiz server: a black-box suite that exercises a **running** instance of
the server through its real public interfaces and asserts only observable
behavior. It is the suite that **must be 100% green before any change ships**.

It is both a **design** (what the harness is, where files live, how scenarios map
to requirements) and an **operations guide** (how to run it locally and in CI, and
what the pass/fail gate means).

**In scope**
- A black-box E2E harness (`godog`) that runs the 47 Gherkin scenarios from
  [`02-test-cases.md`](./02-test-cases.md) against a live server.
- The trigger mechanisms: a manual `make e2e` target and an automatic CI workflow.
- The **pass gate** definition (what blocks a change vs. what only warns).

**Out of scope** (see [§9](#9-out-of-scope))
- Browser/UI automation (the system has no browser front end — there is nothing
  for Playwright to drive).
- Replacing the existing unit / in-process integration tests; E2E sits **above**
  them, it does not duplicate them.

---

## 2. What "E2E" Means For This System

The server exposes two public planes (see [`03-architecture.md`](./03-architecture.md)):

- **Control plane (REST):** `POST /api/sessions`, `…/start`, `…/advance`,
  `…/end`, `…/participants`, `…/answers`, `GET …/leaderboard`, `GET /api/health`,
  `GET /metrics`.
- **Realtime plane (WebSocket):** `GET /ws?quiz_id=&user_id=&name=` — join +
  the server-pushed messages `join_confirmed`, `user_joined`, `question`,
  `score_update`, `leaderboard_update`, `quiz_ended`, `error`.

E2E means: **start a real server process (or container), then act as real clients
against those two planes and assert only what a client can observe.** The harness
must **not** import `internal/domain`, `internal/service`, `internal/store`, or
`internal/handler`. The thing under test is the deployed binary and its wire
contract, end to end.

### 2.1 Position in the test pyramid

| Layer | Where it runs | Imports internal pkgs? | Status |
|-------|---------------|------------------------|--------|
| **Unit** — domain/service/store rules | in-process | yes | ✅ exists |
| **Integration** — handler + real WS via `httptest` | in-process (`httptest.Server`) | yes (`handler`) | ✅ exists (`internal/handler/e2e_test.go`, `ws_integration_test.go`, `ws_timed_test.go`) |
| **E2E (this plan)** — black-box, full process | **separate server process / container** | **no** | ✅ implemented (`tests/e2e/`) |

> **Why a new layer when `internal/handler/e2e_test.go` already exists?** That file
> is valuable but **white-box**: it imports `internal/handler` and runs the handler
> in-process via `httptest`. It cannot catch failures in the *deployment surface* —
> `cmd/server` wiring, env/flag parsing, graceful shutdown, the Docker image,
> health-gated startup, real network framing. The E2E layer closes exactly that gap
> and is the suite CI runs against the shipped artifact.

---

## 3. Harness Architecture

### 3.1 Technology choice

`godog` — the official Cucumber implementation for Go. Rationale:
- The test cases in [`02-test-cases.md`](./02-test-cases.md) are **already written
  as Gherkin** and the README lists "godog BDD feature files" as the planned-but-
  missing piece. godog makes those scenarios *executable* with no rewrite.
- Keeps `docs/02 ↔ features/` **1:1** — the spec and the running gate cannot drift.
- Pure Go, runs under `go test`, no extra language runtime; reuses the
  `gorilla/websocket` dependency already in `go.mod`.

New dependency: `github.com/cucumber/godog` (test-only).

### 3.2 Directory layout

```
features/                         # the 47 scenarios, verbatim from docs/02 (9 files)
├── quiz_session_management.feature
├── quiz_participation.feature
├── score_updates.feature
├── leaderboard.feature
├── connection_reliability.feature
├── session_lifecycle.feature
├── performance_under_load.feature      # Tier 2 (advisory) — tagged @perf
├── input_validation.feature
└── operations_observability.feature

tests/e2e/                        # black-box harness (Go, no internal imports)
├── doc.go                        # package doc (no build tag, so `go build ./...` is happy)
├── main_test.go                  # godog TestMain + runner: readiness gate, tag selection
├── world.go                      # per-scenario state (the "World") + REST/WS helpers
├── client_rest.go                # thin REST client (net/http)
├── client_ws.go                  # thin WS client (gorilla/websocket) + per-type inbox
├── wire.go                       # LOCAL copies of the JSON envelope/payload DTOs
├── steps_ops.go                  # health + metrics
├── steps_session.go              # create/get/join + multi-join + join errors
├── steps_lifecycle.go            # start/advance/end, manual+timed, AFK, end guards
├── steps_score.go                # score updates + broadcast
├── steps_participation.go        # answer correct/incorrect/duplicate/validation
├── steps_leaderboard.go          # ranking + tie-break
├── steps_reliability.go          # disconnect/reconnect + concurrency
├── steps_validation.go           # empty / invalid-option answers
└── steps_perf.go                 # @perf latency + throughput
```

Each `steps_*.go` exposes a `register…Steps(ctx, w)` registered from
`InitializeScenario` in `main_test.go`. All files except `doc.go` carry
`//go:build e2e`; `doc.go` keeps the package non-empty so `go build ./...` and the
tag-less `go test ./...` simply skip it (`[no test files]`).

> **`features/` and the existing empty `features/` dir.** The repo already contains
> an empty `features/` directory reserved for exactly this; the `.feature` files
> land there.

### 3.3 Key design points

**Black-box contract.** `tests/e2e/wire.go` defines **local** structs for the
request/response and WebSocket envelope JSON (mirroring `internal/handler/message.go`
and the REST bodies). The harness never imports `internal/...`. This makes an
accidental wire-format change a **test failure**, which is the point of an E2E gate.

**Configuration / target selection.** The suite reads:
- `E2E_BASE_URL` (default `http://localhost:8080`) — REST base.
- `E2E_WS_URL` (default `ws://localhost:8080`) — WebSocket base.

So the *same* suite runs against `make run` locally, a `docker compose` container,
or a deployed staging URL — no code change.

**Readiness gate.** Before any scenario runs, `TestMain` polls `GET /api/health`
until `200 ok` (bounded retry, ~10s timeout) so the suite never races server
startup. This mirrors the container `HEALTHCHECK` (`server -health`).

**Scenario isolation (parallel-safe).** Each scenario creates **its own quiz
session** with a unique server-generated ID via `POST /api/sessions`; identity is a
per-scenario `user_id` (WS query param) / `X-User-ID` header. No two scenarios
share mutable server state, so they are independent and can be parallelized. The
in-memory store needs no reset between scenarios because sessions never collide.

**The "World".** A per-scenario struct holds: the created `quizId`, a map of named
WS clients (`Alice`, `Bob`, `Charlie`, …) → live connection + a drained inbox of
received messages, and the last REST response/error. Steps read/write the World;
`godog`'s `ScenarioContext.Before/After` constructs it fresh and closes all WS
connections in teardown.

**Timing for `timed` scenarios.** Scenarios that need a per-question timer create
the session with a **short** time limit (e.g. 1–2s, not the doc's illustrative
30s) so auto-advance/auto-complete are exercised quickly and deterministically. The
step text stays human-readable; the World maps "30 seconds" → the configured short
limit.

---

## 4. Scenario → Tier Coverage Matrix

All **47** scenarios are mapped. Two tiers (per the agreed gate policy in [§6](#6-the-100-pass-gate)):

- **Tier 1 — Blocking functional gate.** Must be 100% green to ship.
- **Tier 2 — Advisory.** Run in CI, but a regression posts a **warning**, never a
  red build (latency/throughput/large-concurrency are environment-sensitive on
  shared runners; data-race safety is already a *blocking* gate via `-race` at the
  unit layer).

| Feature file | Scenarios | Tier 1 (blocking) | Tier 2 (advisory) | PRD requirements |
|--------------|-----------|-------------------|-------------------|------------------|
| `quiz_session_management.feature` | 8 | 8 | — | FR-1.1–1.5, AC-1, AC-5 |
| `quiz_participation.feature` | 6 | 6 | — | FR-2.1–2.5 |
| `score_updates.feature` | 5 | 5¹ | — | FR-3.1–3.5, AC-2 |
| `leaderboard.feature` | 5 | 5 | — | FR-4.1–4.6, AC-3 |
| `connection_reliability.feature` | 4 | 2 (disconnect, reconnect) | 2 (`@perf` 100-concurrent submits, concurrent-writes) | NFR-3.1–3.2, AC-4 |
| `session_lifecycle.feature` | 13 | 12 | 1 (`@timing` late-answer → `time_up`) | FR-5.1–5.7, AC-5 |
| `performance_under_load.feature` | 2 | — | 2 (`@perf` latency p95, 50×20 throughput) | NFR-1.1, NFR-2.1–2.3 |
| `input_validation.feature` | 2 | 2 | — | FR-2.3, FR-3 input rules |
| `operations_observability.feature` | 2 | 2 | — | NFR-5.1, NFR-5.3 |
| **Total** | **47** | **42** | **5** | — |

> ¹ **Score-update timing nuance.** In Tier 1, the score scenarios assert the
> broadcast **happens and is correct** (e.g. Alice's `newScore` becomes 20; Bob and
> Charlie *receive* the update). The literal **"within 500 ms"** assertion is
> **measured and reported** (advisory, in `performance_under_load.feature`) — a
> breach warns rather than failing. The correctness of the broadcast is blocking;
> the wall-clock latency on a shared CI runner is not. (Observed locally: p95 ≈ 50µs.)

> **`@timing` reclassification (found during implementation).** The
> *"Rejecting a late answer after a question's time limit expired → `time_up`"*
> scenario is **not deterministically reproducible black-box** and was moved to the
> advisory tier (tagged `@timing`). Because the timed scheduler **auto-advances** at
> expiry, a late answer almost always surfaces as `question_not_found` (the question
> already moved on); `time_up` is only reachable in a sub-millisecond race window.
> The `time_up` rejection **is** verified deterministically at the domain layer
> (`internal/domain` with an injected clock), so coverage is not lost — only the
> black-box reproduction is advisory.

**Tag selection (godog legacy syntax).** Blocking gate runs `~@perf && ~@timing`;
the advisory run uses `@perf,@timing` (comma = OR). `make e2e` and `make e2e-perf`
set these for you.

---

## 5. Running It

### 5.1 Local — manual trigger

```bash
make e2e          # blocking gate: boot server, wait /health, run Tier-1 scenarios, tear down
make e2e-perf     # advisory: run @perf scenarios, print warnings (never fails)
```

`make e2e` orchestration (new Makefile target):
1. Start the server — `docker compose up -d` (or a backgrounded `go run ./cmd/server`).
2. Wait for `GET /api/health` → `200` (readiness gate, bounded retry).
3. `go test -tags e2e ./tests/e2e -run TestE2E -godog.tags='~@perf'`.
4. Tear the server down; surface the godog summary and exit code.

Against an already-running target, skip orchestration:
```bash
E2E_BASE_URL=https://staging.example E2E_WS_URL=wss://staging.example \
  go test -tags e2e ./tests/e2e -godog.tags='~@perf'
```

The E2E package is behind a `//go:build e2e` tag so `make test` (the fast unit/
integration suite) never pays the boot-a-server cost.

### 5.2 CI — automatic trigger

`.github/workflows/e2e.yml`, on every push / pull request:

```
                ┌─────────────────────────────────────────────┐
   push / PR ─► │ build job: go build, docker build elsa-quiz  │
                └───────────────┬─────────────────────────────┘
                                │ image
          ┌─────────────────────┴───────────────────────┐
          ▼                                              ▼
 ┌───────────────────────────┐            ┌──────────────────────────────┐
 │ e2e  (REQUIRED check)      │            │ e2e-perf  (continue-on-error)│
 │ run container              │            │ run container                │
 │ wait /health               │            │ godog --tags=@perf           │
 │ godog --tags=~@perf        │            │ failure ⇒ ⚠ warning annot.   │
 │ any fail ⇒ ❌ red build     │            │ never reds the build         │
 └────────────┬───────────────┘            └───────────────┬──────────────┘
              ▼                                             ▼
   artifacts: godog report (json/pretty), container logs, metrics snapshot
```

- The **`e2e`** job is a **required status check** — a red E2E blocks merge.
- The **`e2e-perf`** job uses `continue-on-error: true`; regressions appear as
  warning annotations on the PR, matching the agreed "warning, not block" policy.
- Both jobs upload artifacts (godog output, server logs, a `/metrics` snapshot) for
  post-mortem.

---

## 6. The 100%-Pass Gate

**Definition of done for any change to this repo:**

| Gate | Command | Blocking? |
|------|---------|-----------|
| Unit + integration tests | `make test` | ✅ yes |
| Data-race freedom | `make test-race` | ✅ yes |
| Lint | `make lint` | ✅ yes |
| **Functional E2E (Tier 1)** | **`make e2e`** | ✅ **yes — 100% of Tier-1 scenarios** |
| E2E perf/load (Tier 2) | `make e2e-perf` | ⚠️ advisory (warns only) |

"100% pass" = **every Tier-1 scenario green**. There is no "mostly passing"
acceptable state for Tier 1; a single failure is a red gate. Tier-2 results are
informational and tracked over time but never block a merge.

---

## 7. Traceability

The coverage matrix in [§4](#4-scenario--tier-coverage-matrix) maps every feature
file to its PRD requirements; [`02-test-cases.md` §Requirements Traceability]
(./02-test-cases.md) already maps each PRD requirement to the named scenarios. The
chain is therefore:

```
PRD requirement (01)  →  Gherkin scenario (02)  →  features/*.feature (verbatim)
                      →  step definition (tests/e2e)  →  blocking/advisory tier (this doc §4)
```

Because the `.feature` files are copied verbatim from `02-test-cases.md`, a drift
between spec and gate is structurally prevented: if a requirement changes, its
scenario changes in `02`, the `.feature` file is updated to match, and the gate
moves with it.

---

## 8. Error-Contract Assertions

E2E pins the public error contract (from
[`backend-implementation/README.md` §5](./backend-implementation/README.md) and
`internal/handler/api.go`). Steps assert both the machine `code` and the HTTP
status mapping:

| Class | Example codes | HTTP |
|-------|---------------|------|
| Not found | `session_not_found`, `question_not_found`, `participant_not_found` | 404 |
| Validation | `quiz_id_required`, `answer_empty`, `invalid_option`, `invalid_time_limit`, `invalid_question` | 400 |
| State conflict | `duplicate_answer`, `session_ended`, `quiz_ended`, `time_up`, `invalid_session_state`, `session_exists` | 409 |

Over WebSocket, the same conditions arrive as an `error` envelope
(`{ "type": "error", "payload": { "code", "message" } }`); steps assert the `code`.

---

## 8a. Deviations from `docs/02-test-cases.md`

The `.feature` files mirror `docs/02` in structure and wording, with these
deliberate, black-box-driven adaptations (each is flagged in a `# NOTE` at the top
of the relevant feature file):

| Area | docs/02 (illustrative) | features/ (executable) | Why |
|------|------------------------|------------------------|-----|
| **Quiz IDs** | literal `QUIZ-ABC`, `QUIZ-OLD` | bound to the server-generated id created in the scenario; `QUIZ-999` stays a real non-existent id; `""` exercises `quiz_id_required` | IDs are server-generated; the harness can't pre-pick them |
| **Timed limits** | 30 seconds | **1 second** (REST contract is whole seconds; sub-second isn't expressible) | keep the suite fast and deterministic |
| **Question counts** | up to 5 | 3 for lifecycle, 5 for score/leaderboard | matched to what each scenario needs |
| **Scores** | absolute presets (30/40/50/60) | reached by *answering N correctly* (+10 each); 60 dropped (unachievable) | black-box scores change only via correct answers |
| **Rejections** | display message text | machine **error code** (`invalid_time_limit`, `quiz_ended`, …) | the code is the stable contract (§8) |
| **`time_up` late answer** | Tier-1 expectation | **advisory `@timing`** | non-deterministic black-box (see §4 note) |
| **Health "active = 2"** | exact count | **"at least N"** + scenario seeds its own sessions | the server is shared across the run; counters are process-global |
| **Metrics `quiz_answers_total`** | assumed present | scenario first **processes an answer** | the counter isn't emitted until first used |
| **Latency / heavy concurrency** | inline | **advisory `@perf`**, counts scaled to the harness WS buffers | load is a separate, non-blocking concern |

Because the scenario *titles and intent* are preserved, the
[traceability](#7-traceability) chain in `docs/02` still holds; only the
illustrative literals changed.

## 9. Out of Scope

- **Browser/UI E2E (Playwright et al.).** No browser front end exists; the realtime
  contract is verified at the WebSocket layer.
- **Authentication flows.** Users are pre-authenticated (PRD §9); identity is a
  provided `user_id`. Nothing to E2E here until auth is built.
- **Persistence/durability across restarts.** The store is in-memory by design for
  the MVP; restart-survival becomes an E2E concern when a Redis/PG store lands.
- **Multi-instance / horizontal-scale fan-out.** A scale concern for when a shared
  pub/sub backplane is added; out of scope for the single-instance MVP.

---

## 10. Implementation Status

✅ **Implemented** — built test-first, increment by increment, each kept green:

1. ✅ **Scaffold** `tests/e2e/` (build-tagged) + `godog` dep + `make e2e` +
   readiness gate + World — proven with the `operations_observability` health/metrics
   scenarios.
2. ✅ **Control-plane steps** — `quiz_session_management` (8) + `session_lifecycle`
   (12 Tier-1).
3. ✅ **Realtime steps** — `quiz_participation` (6), `score_updates` (5),
   `leaderboard` (5), and the 2 Tier-1 `connection_reliability` scenarios.
4. ✅ **Validation + error contract** — `input_validation` (2) + negative cases.
5. ✅ **Tier-2 `@perf`/`@timing`** — `performance_under_load` (2) + 2 reliability
   concurrency + 1 `@timing`; `make e2e-perf` + `.github/workflows/e2e.yml`.

**Definition of done — met:** `make e2e` exits 0 with **42/42** Tier-1 scenarios
passing; `make e2e-perf` runs warn-only (exits 0 even when the `@timing` scenario
flakes). Verified locally on Go 1.26.1 / godog 0.15.1.

---

## 11. AI Collaboration Documentation

- **Tool:** Claude (Anthropic) — `verification-before-completion`, `brainstorming`,
  and the `e2e-testing` / `e2e-testing-patterns` skills.
- **Task:** Verify the current build state (build/vet/test/-race/lint/coverage all
  green) before planning; then design a requirements-traceable E2E plan.
- **Verification of the AI output that produced this plan:** the project's gates
  were run fresh — `go build ./...` (exit 0), `go vet ./...` (exit 0),
  `go test ./...` (all ok), `go test -race ./...` (all ok),
  `golangci-lint run ./...` (0 issues), coverage domain 94.8% / service 93.9% /
  store 100% / handler 83.5%. The harness design was reconciled against the actual
  route table in `internal/handler/api.go`, the WS message catalog in
  `internal/handler/message.go`, and the error-code mapping, so every endpoint and
  message the plan references is confirmed to exist in the code.
- **Verification of the implementation (v1.1):** built test-first with `golang-ticket-tdd`,
  running the godog suite after every feature increment and keeping it green.
  Final gates: `make e2e` → exit 0 (**42/42** Tier-1), `make e2e-perf` → exit 0
  (advisory; p95 ≈ 50µs, throughput 1000/1000), `go test ./...` and
  `go test -race ./...` unaffected (e2e excluded without the build tag),
  `golangci-lint run ./...` (0) and `golangci-lint run --build-tags e2e ./tests/e2e/...`
  (0). Two findings surfaced *during* implementation and are folded back into this
  doc: the `time_up` non-determinism (now `@timing` advisory) and the
  shared-server counter semantics (health/metrics assert thresholds + seed activity).
  Full hand-off in [`tickets-details/e2e-godog-harness/`](../tickets-details/e2e-godog-harness/).
