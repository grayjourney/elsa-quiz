# Backend Implementation Guide — Real-Time Vocabulary Quiz

> A single, self-contained walkthrough of the backend: what it does, how it is
> structured, **how data flows through it step by step**, the design decisions
> behind it, and how to verify it locally. If you've just read `test.md` and want
> to understand this codebase quickly, read this top to bottom.

**Module:** `github.com/gray/elsa-quiz` · **Go:** 1.22+ (developed on 1.26)
**Built:** strictly test-first (RED → GREEN → REFACTOR)

---

## Table of contents

1. [The big picture](#1-the-big-picture)
2. [Architecture in layers](#2-architecture-in-layers)
3. [Workflow walkthroughs (step by step)](#3-workflow-walkthroughs-step-by-step)
4. [The code, file by file](#4-the-code-file-by-file)
5. [Public API surface (Go)](#5-public-api-surface-go)
6. [Key behavior rules](#6-key-behavior-rules)
7. [Design decisions & alternatives](#7-design-decisions--alternatives)
8. [Local testing](#8-local-testing)
9. [What's next](#9-whats-next)

---

## 1. The big picture

The server lets many players compete in the same quiz in real time:

```
Players (WebSocket clients)
        │  join, submit answers
        ▼
┌───────────────────────────────────────────┐
│  HTTP / WebSocket layer   (next increment) │
├───────────────────────────────────────────┤
│  service     SessionService ScoringService │   orchestration
│              LeaderboardService            │
├───────────────────────────────────────────┤
│  store       Store (interface) + Memory    │   persistence seam
├───────────────────────────────────────────┤
│  domain      QuizSession (aggregate root)  │   all business rules
│              Question Participant Score     │
│              Leaderboard Errors            │
└───────────────────────────────────────────┘
```

Each layer only knows about the layer beneath it. **All business rules live in
`domain`** and have zero I/O, so they are fast and exhaustively unit-tested. The
implementation now spans the **full stack**: `domain`, `store`, `service`, and the
`handler` layer (REST + WebSocket), wired together in `cmd/server`. The runnable
server exposes the REST control/query API, the real-time WebSocket channel,
Prometheus `/metrics`, and pprof.

---

## 2. Architecture in layers

| Layer | Package | Responsibility | Depends on |
|-------|---------|----------------|-----------|
| Domain | `internal/domain` | Entities and **all invariants**: validation, scoring, ranking, the quiz state machine, end policies, concurrency safety | nothing |
| Store | `internal/store` | Persist & retrieve sessions behind an interface | `domain` |
| Service | `internal/service` | Orchestrate use-cases (create/join/start, submit→score→leaderboard) | `domain`, `store` |
| ID | `pkg/id` | Generate unique, URL-safe IDs | nothing |
| Handler | `internal/handler` | REST + WebSocket transport, connection manager, broadcasting, Prometheus metrics | `service` |
| Server | `cmd/server` | Wiring, `slog`, graceful shutdown, `-health` self-check | `handler` |

**Why this shape?** Clean architecture (ADR-001/004): the core is independent of
transport and storage, so we can unit-test rules without a network or DB, and
swap the in-memory store for Redis later by implementing one interface.

---

## 3. Workflow walkthroughs (step by step)

These trace a real request through the layers. They are the fastest way to learn
the codebase.

### 3.1 Creating a quiz session

```
Host → SessionService.CreateSession(questions, policy, timeLimit)
  1. id.New()                    → "QUIZ-" + crypto-random base32 code
  2. domain.NewQuizSession(...)  → validates: timed policy requires timeLimit > 0
                                   → returns aggregate in status "waiting", currentIdx = -1
  3. store.CreateSession(s)      → persists; rejects a duplicate ID (ErrSessionExists)
  4. return *QuizSession         → caller learns the generated quizId
```

Result: a `waiting` session that players can join. If `policy == "timed"` and
`timeLimit <= 0`, step 2 returns `ErrInvalidTimeLimit` and nothing is stored.

### 3.2 A player joining

```
Player → SessionService.JoinSession(quizID, userID, displayName)
  1. guard: quizID == ""         → ErrQuizIDRequired
  2. store.GetSession(quizID)    → ErrSessionNotFound if missing
  3. domain.NewParticipant(...)  → Score 0, empty answered-set, JoinedAt now
  4. session.AddParticipant(p)   → ErrSessionEnded if the session is completed
                                   → otherwise registers the participant (under the
                                     session's mutex)
  5. return *Participant
```

Joining is allowed while `waiting` **or** `active` (late-join, FR-2.5). Only a
`completed` session refuses new players.

### 3.3 Starting the quiz

```
Host → SessionService.StartSession(quizID, at)
  1. store.GetSession(quizID)
  2. session.Start(at)
       - guard: status must be "waiting" else ErrInvalidSessionState
       - status → "active", currentIdx → 0, openedAt → at (the first question opens)
```

### 3.4 Submitting an answer — the hot path

This is the most important flow. The `ScoringService` is a thin coordinator; the
**aggregate does the work atomically under its own lock**:

```
Player → ScoringService.SubmitAnswer(quizID, userID, questionID, answer, at)
  1. store.GetSession(quizID)                         → ErrSessionNotFound
  2. session.SubmitAnswer(userID, questionID, answer, basePoints, at):
       a. lock the session (one critical section guards the whole operation)
       b. status completed?      → ErrQuizEnded
          status not active?     → ErrInvalidSessionState
       c. participant exists?    → else ErrSessionNotFound (unknown user)
       d. questionID == current? → else ErrQuestionNotFound
                                    (this also closes past questions)
       e. timed & past limit?    → ErrTimeUp
       f. answer == ""?          → ErrAnswerEmpty
          answer not an option?  → ErrInvalidOption
          already answered?      → ErrDuplicateAnswer
       g. record the answer (prevents resubmission)
       h. points = CalculateScore(isCorrect, basePoints)
          if points > 0: participant.AddScore(points, at)   ← stamps LastScoredAt
       i. return AnswerResult{IsCorrect, AwardedPoints, NewScore}
  3. leaderboard = CalculateLeaderboard(session.Participants())
  4. return (result, leaderboard)
```

The future WebSocket handler will take `result` + `leaderboard` and broadcast
`score_update` and `leaderboard_update` to every connected client. Because the
critical section is a single lock, **100 players submitting at once cannot race
or lose a score** — proven by `TestQuizSession_ConcurrentSubmissions_NoRaceNoLostScores`.

### 3.5 Reading the leaderboard

```
Anyone → LeaderboardService.GetLeaderboard(quizID)
  1. store.GetSession(quizID)                  → ErrSessionNotFound
  2. CalculateLeaderboard(session.Participants())
       - snapshot participants (value copies, taken under the lock)
       - sort: Score DESC, then LastScoredAt ASC, then UserID ASC
       - assign ranks 1..N
```

The leaderboard is **always recomputed from current scores** — there is no
separate structure to keep in sync, so it can never drift.

### 3.6 Ending a quiz (the AFK-liveness fix)

Two end policies, both immune to absent players:

- **manual:** the host calls advance/end. `AdvanceQuestion(at)` moves to the next
  question; advancing past the last one auto-completes the session. `Complete()`
  ends it immediately. Missed questions simply score 0.
- **timed:** each question has a limit; a submission after `openedAt + TimeLimit`
  is rejected with `ErrTimeUp`. The handler's timer advances the question when the
  limit elapses, so one idle player never blocks the room.

---

## 4. The code, file by file

All files are new (greenfield). Layout follows `docs/03-architecture.md §5`.

### Production code

| File | What it contains |
|------|------------------|
| `pkg/id/generator.go` | `id.New()` — 10 random bytes → base32 (no padding) → short URL-safe ID. No dependencies. |
| `internal/domain/errors.go` | `domain.Error{Code, Message}` and 14 sentinels. `Code` is machine-readable; `Message` is the exact user-facing string from the test cases. Matched with `errors.Is` through wrapping. |
| `internal/domain/quiz.go` | `Question` (`Validate`, `IsCorrectAnswer`, `HasOption`) **and** the `QuizSession` aggregate: status/end-policy/time-limit, participant registry, `Start`/`AdvanceQuestion`/`Complete`/`SubmitAnswer`, a `sync.Mutex` guarding all access. |
| `internal/domain/participant.go` | `Participant`: identity, `Score`, `LastScoredAt`, `JoinedAt`, a private answered-set; `AddScore`, `HasAnswered`, `RecordAnswer` (duplicate guard). |
| `internal/domain/score.go` | `CalculateScore(isCorrect, basePoints)` — pure function. |
| `internal/domain/leaderboard.go` | `LeaderboardEntry` and `CalculateLeaderboard` — pure ranking with deterministic tie-break. |
| `internal/store/store.go` | `Store` interface: `CreateSession`, `GetSession`, `ActiveSessionCount`. |
| `internal/store/memory_store.go` | `MemoryStore` — `sync.RWMutex` + `map[string]*QuizSession`; not-found → `domain.ErrSessionNotFound`, duplicate → `domain.ErrSessionExists`. |
| `internal/service/session_service.go` | Create / join / start orchestration. |
| `internal/service/scoring_service.go` | Submit → score → recompute leaderboard. |
| `internal/service/leaderboard_service.go` | Read current leaderboard. |
| `internal/handler/api.go` | REST routes, handlers, error→HTTP-status mapping, pprof, `/metrics`. |
| `internal/handler/message.go` | WebSocket message envelope + typed payloads. |
| `internal/handler/connection_manager.go` | Per-session connection registry + broadcast (one writer goroutine per client). |
| `internal/handler/ws_handler.go` | WS upgrade, join, read loop, broadcast on answer. |
| `internal/handler/metrics.go` | Prometheus collectors + `/metrics` handler. |
| `cmd/server/main.go` | Wiring, `slog`, graceful shutdown, `-health` flag. |

### Test code (24 test functions)

| File | Covers |
|------|--------|
| `pkg/id/generator_test.go` | Non-empty + 1000-way uniqueness |
| `internal/domain/errors_test.go` | Each sentinel's code/message; `errors.Is` through wrapping |
| `internal/domain/quiz_test.go` | `Question.Validate` (6-case table), `IsCorrectAnswer`, `HasOption` |
| `internal/domain/participant_test.go` | Zero-score init, `AddScore`, `LastScoredAt` stamping, duplicate guard |
| `internal/domain/score_test.go` | Correct/incorrect/variable base points |
| `internal/domain/leaderboard_test.go` | Rank order, time tie-break, empty, single, full-tie determinism |
| `internal/domain/quiz_session_test.go` | Lifecycle, advance/complete, `SubmitAnswer` happy + 6 error paths, timed time-up |
| `internal/domain/quiz_session_concurrency_test.go` | 100 concurrent submits — no race, no lost scores (`-race`) |
| `internal/store/memory_store_test.go` | Create/get, not-found, duplicate, active count, concurrent access |
| `internal/service/session_service_test.go` | Create/join/start + join error paths |
| `internal/service/scoring_service_test.go` | Submit happy/incorrect/errors + leaderboard services |

---

## 5. Public API surface (Go)

```go
// pkg/id
func New() string

// internal/domain
type Error struct{ Code, Message string }            // sentinels, errors.Is-matchable
func CalculateScore(isCorrect bool, basePoints int) int
func CalculateLeaderboard(participants []Participant) []LeaderboardEntry

type Question struct { ID, Text string; Options []string; CorrectAnswer string; Order int }
func (q Question) Validate() error
func (q Question) IsCorrectAnswer(answer string) bool
func (q Question) HasOption(answer string) bool

type Participant struct { UserID, SessionID, DisplayName string; Score int; LastScoredAt, JoinedAt time.Time }
func NewParticipant(userID, sessionID, displayName string) *Participant
func (p *Participant) AddScore(points int, at time.Time)
func (p *Participant) HasAnswered(questionID string) bool
func (p *Participant) RecordAnswer(questionID string) error

type EndPolicy string        // "manual" | "timed"
type SessionStatus string    // "waiting" | "active" | "completed"
type AnswerResult struct { IsCorrect bool; AwardedPoints, NewScore int }

type QuizSession struct { /* ID, Status, EndPolicy, TimeLimit, Questions, CreatedAt, ... */ }
func NewQuizSession(id string, questions []Question, policy EndPolicy, timeLimit time.Duration) (*QuizSession, error)
func (s *QuizSession) AddParticipant(p *Participant) error
func (s *QuizSession) Participant(userID string) (*Participant, bool)
func (s *QuizSession) Participants() []Participant
func (s *QuizSession) GetStatus() SessionStatus
func (s *QuizSession) Start(at time.Time) error
func (s *QuizSession) CurrentQuestion() (Question, bool)
func (s *QuizSession) AdvanceQuestion(at time.Time) (Question, bool, error)
func (s *QuizSession) SubmitAnswer(userID, questionID, answer string, basePoints int, at time.Time) (AnswerResult, error)
func (s *QuizSession) Complete()

// internal/store
type Store interface {
	CreateSession(s *domain.QuizSession) error
	GetSession(id string) (*domain.QuizSession, error)
	ActiveSessionCount() int
}
func NewMemoryStore() *MemoryStore

// internal/service
func NewSessionService(st store.Store) *SessionService
func (svc *SessionService) CreateSession(questions []domain.Question, policy domain.EndPolicy, timeLimit time.Duration) (*domain.QuizSession, error)
func (svc *SessionService) JoinSession(quizID, userID, displayName string) (*domain.Participant, error)
func (svc *SessionService) StartSession(quizID string, at time.Time) error

func NewScoringService(st store.Store, basePoints int) *ScoringService
func (svc *ScoringService) SubmitAnswer(quizID, userID, questionID, answer string, at time.Time) (domain.AnswerResult, []domain.LeaderboardEntry, error)

func NewLeaderboardService(st store.Store) *LeaderboardService
func (svc *LeaderboardService) GetLeaderboard(quizID string) ([]domain.LeaderboardEntry, error)
```

### Transport endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/health` | Liveness + active-session count |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/debug/pprof/…` | Profiling |
| `POST` | `/api/sessions` | Create session |
| `GET` | `/api/sessions/{id}` | Session state |
| `POST` | `/api/sessions/{id}/participants` | Join |
| `POST` | `/api/sessions/{id}/start` · `/advance` · `/end` | Host controls (broadcast over WS) |
| `POST` | `/api/sessions/{id}/answers` | Submit answer (REST mirror of the WS message) |
| `GET` | `/api/sessions/{id}/leaderboard` | Current leaderboard |
| `GET` | `/ws?quiz_id=&user_id=&name=` | WebSocket: join + real-time messaging |

**WebSocket messages** (envelope: `{"type": "...", "payload": {...}}`):

- client → server: `submit_answer`
- server → client: `join_confirmed`, `user_joined`, `question`, `score_update`,
  `leaderboard_update`, `quiz_ended`, `error`

Host actions (`start`/`advance`/`end`) are REST calls; the server pushes the
resulting `question` / `quiz_ended` to all connected clients. Full payload shapes
and samples are in [`docs/postman/`](../postman/).

The full error catalog (`Code` → `Message`) used across the API:

| Code | Message |
|------|---------|
| `session_not_found` | Quiz session not found |
| `session_ended` | Quiz session has already ended |
| `session_exists` | Quiz session already exists |
| `quiz_id_required` | Quiz ID is required |
| `question_not_found` | Question not found |
| `duplicate_answer` | Answer already submitted for this question |
| `answer_empty` | Answer cannot be empty |
| `invalid_option` | Invalid answer option |
| `quiz_ended` | Quiz has already ended |
| `time_up` | Time is up for this question |
| `invalid_time_limit` | Time limit must be greater than zero |
| `invalid_question` | Question is invalid |
| `invalid_session_state` | Operation not allowed in the current session state |
| `participant_not_found` | Participant has not joined this quiz session |

---

## 6. Key behavior rules

- **Answers apply only to the current question.** A mismatched/unknown question
  ID → `ErrQuestionNotFound`; this also closes past questions after advancing.
- **`LastScoredAt` is stamped only when points are awarded** (correct answer).
  Incorrect answers are recorded (to block resubmission) but don't move the
  tie-break timestamp — so tie-breaking reflects *when a score was reached*.
- **End policies never depend on a specific participant.** `manual` = host
  controls; `timed` = the clock controls. AFK players score 0 and the quiz moves on.
- **Concurrency.** The `QuizSession` aggregate owns a `sync.Mutex`; every mutating
  and reading method is guarded; status is read via `GetStatus()` so the store
  never races on the `Status` field. Lock ordering is always store → session.
- **Determinism.** Time is injected (`at time.Time`) everywhere, so scoring,
  tie-breaking, and time limits are reproducible in tests.

---

## 7. Design decisions & alternatives

| Decision | Chosen | Alternative | Verdict |
|----------|--------|-------------|---------|
| Where invariants live | **Aggregate root** (`QuizSession` owns submit/lifecycle) | Anaemic structs + logic in services | Chosen — state transitions stay atomic and can't be bypassed; services stay thin |
| Concurrency control | **Mutex inside the aggregate** | Lock per session in the service/store | Chosen — the owner of the state owns its safety; one consistent lock order |
| Error model | **Typed `Error{Code,Message}` sentinels** | `errors.New` strings / separate display map | Chosen — machine code *and* exact user message in one place; avoids the `ST1005` capitalized-string lint trap |
| ID generation | **stdlib `crypto/rand` base32** | `google/uuid` (named in plan) | Chosen — zero dependency, shorter human-typable join codes (**deviation from docs**) |
| Time source | **Injected `at time.Time`** | `time.Now()` inside methods | Chosen — deterministic tests for scoring, ties, time limits |
| Leaderboard | **Pure recompute from snapshots** | Incrementally maintained sorted structure | Chosen — at ≤100/session it's trivially fast and obviously correct |

**Deviations from the planning docs**
- ID generator uses `crypto/rand` instead of `google/uuid` (worth a one-line note
  in `docs/03-architecture.md §4.1`).
- The `manual`/`timed` end-policy logic is implemented ahead of a formal PRD
  **FR-5**; `docs/02-test-cases.md` already specifies it. Sync PRD/architecture
  when convenient.

---

## 8. Local testing & running

**Run the server** (see the root [`README.md`](../../README.md) for full setup):
```bash
make run        # native on :8080  (or `make up` for the full Docker stack)
curl localhost:8080/api/health
```

**Run the tests** (unit + real-WebSocket integration):
```bash
make test          # all packages green
make test-race     # concurrency-safe (no races)
make lint          # 0 issues
make test-cover    # coverage report
```

Expected coverage: domain ~93.7%, service ~93.9%, store 100%, handler ~82%, id 75%
(only the unreachable CSPRNG-panic branch is uncovered).

The `handler` package includes a real-WebSocket integration test
(`ws_integration_test.go`) that dials two clients, starts a quiz, submits an
answer, and asserts both clients receive `score_update` + `leaderboard_update` —
the end-to-end real-time path — and an `e2e_test.go` that walks the
`docs/02-test-cases.md` scenarios against a live server.

Targeted runs:

```bash
go test -race -run TestQuizSession_ConcurrentSubmissions ./internal/domain/ -v
go test -run TestCalculateLeaderboard ./internal/domain/ -v
go test -run 'TestScoringService|TestSessionService' ./internal/service/ -v
```

**Mapping to `docs/02-test-cases.md`:**

| Test-case feature | Unit tests |
|-------------------|------------|
| Quiz Session Management | `session_service_test.go`, `quiz_session_test.go` |
| Real-Time Score Updates | `score_test.go`, `scoring_service_test.go`, `quiz_session_test.go` |
| Real-Time Leaderboard | `leaderboard_test.go`, `scoring_service_test.go` |
| Concurrency / no races | `quiz_session_concurrency_test.go`, `memory_store_test.go` |
| Session Lifecycle + end policies | `quiz_session_test.go` (manual/timed, advance, complete, time-up) |

The end-to-end WebSocket scenarios (broadcast/latency) and `godog` BDD files are
validated in the next increment. The HTTP/WebSocket **contract** they will satisfy
is already published as a Postman collection — see [`docs/postman/`](../postman/).

---

## 9. What's next

The core challenge is complete (real-time participation, scoring, leaderboard over
WebSocket; REST control API; Docker; observability). Remaining nice-to-haves:

| Item | Notes |
|------|-------|
| `godog` BDD feature files | Optional — the `docs/02-test-cases.md` scenarios are already covered by table-driven unit tests + the `e2e_test.go` integration suite |
| Grafana dashboard JSON | Datasource is provisioned; a pre-built dashboard panel set could be added |
| Redis-backed store + pub/sub | Designed for (the `Store` interface + broadcast seam); enables horizontal scaling |
| PRD/architecture sync | Formalize **FR-5 Quiz End Policy** and note the `crypto/rand` ID choice |
