# Postman Collection — Real-Time Vocabulary Quiz API

A contract-first Postman collection covering **every testable scenario** in
[`docs/02-test-cases.md`](../02-test-cases.md). Each request maps to one or more
Gherkin scenarios, with full headers, payloads, auth notes, sample responses, and
`pm.test` assertions.

## Files
| File | Purpose |
|------|---------|
| `Real-Time-Quiz.postman_collection.json` | The collection (import into Postman) |
| `Real-Time-Quiz.postman_environment.json` | Local environment (`baseUrl`, `wsUrl`, ids) |

## Import
1. Postman → **Import** → drop both JSON files.
2. Select the **Real-Time Quiz — Local** environment (top-right).
3. Ensure the server is running on `localhost:8080` (next implementation increment).

> **Status:** the server is **implemented** — start it with `make run` (native) or
> `make up` (Docker), then **Run collection** executes the REST suite against
> `localhost:8080`. The real-time WebSocket channel (join + broadcasts) is in the
> **Realtime (WebSocket)** folder; open it as a Postman WebSocket request, or see
> the end-to-end flow validated automatically in `internal/handler/ws_integration_test.go`.

## How it's organized

| Folder | Scenarios covered (from `docs/02-test-cases.md`) |
|--------|--------------------------------------------------|
| Health & Observability | server-running background, metrics (NFR-5) |
| Session Management | create (manual/timed/invalid-limit), get session, not-found |
| Participation - Join | join valid/non-existent/completed, notify on join |
| Session Lifecycle | start, advance, end (incl. AFK), end-already-completed |
| Participation - Answers | correct, incorrect, duplicate, unknown question, empty, invalid option, after-end, time-up |
| Leaderboard | ranking + tie-break |
| Realtime (WebSocket) | join/connect, question/score/leaderboard broadcasts, errors |

> **Timed sessions auto-advance.** For a `timed` session the server pushes the next
> `question` (and finally `quiz_ended`) over WebSocket on its own — when the
> per-question timer expires, or early once all connected players have answered. No
> client/host action is needed between questions. The "Late answer in timed quiz →
> 409 time_up" request still applies to a question whose window has closed.

## Running as a suite
The HTTP requests chain via collection variables: **Create session** saves
`quizId`; **Join** saves `userId`/`otherUserId`. A natural run order:

1. Health check
2. Create session (manual) → Join Alice → Join Bob → Start quiz
3. Submit correct (Alice) → Submit incorrect (Bob) → Duplicate (Alice)
4. Get leaderboard
5. End quiz → Answer after ended

Negative cases (invalid limit, unknown session/question, empty/invalid answer)
are self-contained and can run any time.

## Conventions

- **Auth:** none (mocked). Identity is `user_id` (WS query param) or the
  `X-User-ID` header (HTTP). No tokens.
- **Error body:** `{ "code": "<machine_code>", "message": "<user message>" }` —
  the full catalog is in
  [`docs/backend-implementation/README.md` §5](../backend-implementation/README.md#5-public-api-surface-go).
- **HTTP status mapping:** `*_not_found` → 404; validation (`answer_empty`,
  `invalid_option`, `invalid_time_limit`, `quiz_id_required`, `invalid_question`)
  → 400; state conflicts (`duplicate_answer`, `session_ended`, `quiz_ended`,
  `time_up`, `invalid_session_state`, `session_exists`) → 409.

## What can't be asserted in Postman
Latency-percentile and high-throughput/concurrency scenarios (broadcast < 500ms
at p95, 100 simultaneous submits, 50×20 sessions) are load/perf concerns. They
are verified by the Go `-race` tests and will be covered by load tests in the
next increment — not by this functional collection.
