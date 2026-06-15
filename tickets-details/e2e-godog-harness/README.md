# e2e-godog-harness — Black-box end-to-end test suite

**Type:** Feature (test infrastructure)
**Branch:** `feature/e2e-godog-harness`
**Outcome:** A runnable, black-box e2e gate (`godog`) covering the 47 Gherkin
scenarios against a *running* server over real WebSocket + HTTP. `make e2e` is
green at **42/42** Tier-1 scenarios; `make e2e-perf` runs the advisory tier.

> Design, tier policy, coverage matrix and deviations live in
> [`docs/05-e2e-test-plan.md`](../../docs/05-e2e-test-plan.md). This hand-off is the
> "what changed and why" companion.

---

## 1. Goal

Turn the spec's 47 Gherkin scenarios (`docs/02-test-cases.md`) into an automated
gate that runs against a **deployed** server (not in-process), asserting only what
a real client observes — closing the gap left by the existing in-process
`internal/handler` tests (which can't catch `cmd/server` wiring, env/flag parsing,
real network framing, or the shipped binary). The gate must be 100%-pass for
functional scenarios and triggerable both manually (`make e2e`) and in CI.

## 2. Changes summary

| File | Status | What |
|------|--------|------|
| `features/*.feature` (9) | new | The 47 scenarios as executable Gherkin (verbatim intent; black-box adaptations noted inline) |
| `tests/e2e/doc.go` | new | Package doc with **no** build tag, so `go build ./...` / tag-less `go test ./...` skip the package cleanly |
| `tests/e2e/main_test.go` | new | godog `TestMain` (health readiness gate) + runner + `InitializeScenario` (fresh World per scenario) |
| `tests/e2e/world.go` | new | Per-scenario state + REST/WS helpers (create/join/start/advance/end/submit, leaderboard, concurrency, latency) |
| `tests/e2e/client_rest.go` | new | Thin `net/http` client |
| `tests/e2e/client_ws.go` | new | `gorilla/websocket` client with a per-type inbox (`waitFor`) + failed-handshake error capture |
| `tests/e2e/wire.go` | new | **Local** copies of the JSON contract (no `internal/...` import) |
| `tests/e2e/steps_*.go` (8) | new | Step definitions grouped by feature area |
| `scripts/e2e.sh` | new | Orchestration: build → boot server → wait `/health` → run godog → tear down |
| `Makefile` | modified | `e2e` (blocking) + `e2e-perf` (advisory) targets |
| `.github/workflows/e2e.yml` | new | Required `e2e` job + `continue-on-error` `e2e-perf` job + log artifacts |
| `go.mod` / `go.sum` | modified | Added `github.com/cucumber/godog v0.15.1` (test-only) |
| `.gitignore` | modified | Ignore `e2e-server.log` |
| `docs/05-e2e-test-plan.md` | modified | Status → implemented; tier counts 42/5; `§8a Deviations`; `@timing` note |
| `docs/02-test-cases.md`, `README.md` | modified | Point at the now-executable suite |

## 3. Why these files

- **`features/` + `tests/e2e/` split** mirrors godog's model: Gherkin (the spec)
  vs. step definitions (the glue). Keeping `features/` 1:1 with `docs/02` means the
  spec and the gate can't silently drift.
- **`wire.go` re-declares the contract** instead of importing `internal/handler`.
  That's the whole point of a black-box gate: an accidental server-side JSON rename
  becomes a *failing scenario*, not a silently-still-compiling test.
- **`doc.go` (untagged)** keeps the package non-empty so `go build ./...` doesn't
  error with "build constraints exclude all Go files," while every other file is
  `//go:build e2e` so the fast unit pass never boots a server.
- **`scripts/e2e.sh`** centralizes orchestration so `make e2e` and CI share one
  readiness-gated path; the suite also runs standalone against any
  `E2E_BASE_URL` / `E2E_WS_URL` (local, container, or staging).

## 4. Solution applied

Built test-first, one feature at a time, running the godog suite after each
increment and keeping it green (RED = scenario undefined/failing → GREEN = step
defs + correct server behavior). Increment order: ops health → session mgmt →
lifecycle → score → participation → leaderboard → reliability → validation →
`@perf`. Each scenario creates its **own** session (unique server id), so scenarios
are isolated and parallel-safe with no shared mutable state.

**Tiers / gate:** `~@perf && ~@timing` = blocking (42 scenarios, `make e2e`);
`@perf,@timing` = advisory (`make e2e-perf`, never fails the build).

## 5. Why this solution (alternatives considered)

- **godog vs. a pure-Go black-box `_test.go` suite.** godog won because the spec is
  *already* Gherkin and the README called out the missing "godog BDD feature files"
  — godog makes the spec executable with no rewrite and preserves traceability. A
  hand-rolled Go suite would have duplicated the scenarios in code, untraceable.
- **godog vs. Newman/Postman.** Newman can replay the existing REST collection but
  can't meaningfully assert WebSocket broadcasts or concurrency — the realtime core
  is the whole point, so REST-only was insufficient.
- **Local DTOs vs. importing `internal/handler`.** Importing would delete `wire.go`
  but make the test blind to contract breakage — it would no longer be black-box.
  Rejected on purpose.
- **Blocking `time_up` vs. advisory.** Forcing the late-answer→`time_up` scenario
  into the blocking gate would make it flaky (the timed scheduler auto-advances, so
  the error is usually `question_not_found`). Moved to advisory `@timing`; the
  deterministic `time_up` proof stays at the domain layer.

## 6. Findings surfaced during implementation

1. **REST answer submission does not broadcast over WebSocket** — only the WS
   `submit_answer` path does. So score/leaderboard *broadcast* scenarios submit over
   WS; REST is used where only the caller's result matters. (Server behavior is
   correct; this just shaped the harness.)
2. **`time_up` is not deterministically reproducible black-box** (auto-advance
   race) → reclassified `@timing` advisory; covered deterministically in
   `internal/domain` with an injected clock.
3. **Shared-server counters are process-global** → health/metrics scenarios assert
   "at least N" and seed their own activity (e.g. process one answer so
   `quiz_answers_total` is emitted).

These are documented in `docs/05` §4 and §8a.

## 7. Manual test guide

Prereqs: Go 1.22+ (developed on 1.26), `make`. No Docker needed for the native path.

```bash
# 1) Existing gates still pass and DO NOT run e2e (no build tag):
make test           # ok; tests/e2e shows [no test files]
make test-race      # ok
make lint           # 0 issues

# 2) Blocking e2e gate (builds + boots server on :8090, runs Tier-1, tears down):
make e2e            # expect: exit 0, godog "ok"

# 3) Advisory tier (perf + timing); never fails the build:
make e2e-perf       # expect: exit 0; prints [perf] p95 + throughput; may WARN on @timing

# 4) Verbose scenario-by-scenario (against a server you start yourself):
go build -o bin/quiz ./cmd/server
PORT=8090 ./bin/quiz &
E2E_BASE_URL=http://localhost:8090 E2E_WS_URL=ws://localhost:8090 \
  E2E_TAGS='~@perf && ~@timing' go test -tags e2e -v ./tests/e2e
kill %1

# 5) Run one feature:
#   ... same as (4) but add: -godog.paths ../../features/leaderboard.feature  (or set E2E_TAGS)

# 6) Run against a remote/staging target (no orchestration):
E2E_BASE_URL=https://staging.example E2E_WS_URL=wss://staging.example \
  go test -tags e2e ./tests/e2e
```

**Expected (local, latest run):** `make e2e` → `42 scenarios (42 passed)`, exit 0;
`make e2e-perf` → `[perf] broadcast latency p95≈50µs`, `throughput accepted=1000/1000`,
exit 0 (the `@timing` late-answer scenario warns, by design).

**Env vars:** `E2E_BASE_URL` (default `http://localhost:8080`), `E2E_WS_URL`
(default `ws://localhost:8080`), `E2E_TAGS` (godog tag expression), `E2E_PORT`
(server port for the `make` targets, default `8090`), `GODOG_FORMAT` (default
`pretty`).
