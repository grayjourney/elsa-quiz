# Web Client (bonus)

**Version:** 1.0
**Date:** 2026-06-16
**Status:** ✅ Implemented

A single-file web client for the Real-Time Quiz, so two players can join by code and
compete live over WebSocket without any tooling. It is served by the quiz server
itself, so there is nothing to install or build.

---

## 1. What it is

One `web/index.html` (vanilla JS + CSS, no framework, no build step) embedded into
the Go binary with `go:embed` and served at `GET /`. Open `http://localhost:8080`
in two browser windows and play head to head: lobby → game → results, with a live
leaderboard and a countdown ring on timed games.

It talks to the same backend everything else uses: REST for control
(`/api/sessions…`) and a WebSocket (`/ws`) for the real-time plane. Same origin, so
no CORS and no configuration.

## 2. How to run

```bash
make run        # native: server + web client on http://localhost:8080
# or
make up         # Docker: same, the page ships inside the image
```

Then open **http://localhost:8080** in two windows.

- **Window 1 (host):** enter a name, **Create a quiz**, choose **Manual** or
  **Timed** (seconds per question), **Create & host**. A quiz **code** appears.
- **Window 2 (player):** enter a name, **Join with code**, paste the code.
- **Host** clicks **Start**. In *manual* the host also drives **Next** / **End**;
  in *timed* questions advance and the quiz ends on the clock.
- Both windows answer by clicking an option; scores and the leaderboard update the
  instant anyone answers.

## 3. Screens

| Screen | What it shows |
|--------|---------------|
| **Lobby** | name; Create (policy + seconds) or Join (code) |
| **Game** | question + options, countdown ring (timed), host controls, live leaderboard sidebar |
| **Results** | final ranking and winner, "Play again" |

## 4. How it maps to the backend

| UI action | Backend call |
|-----------|--------------|
| Create & host | `POST /api/sessions` (built-in 5-question vocab set) |
| Join / host join | WebSocket `GET /ws?quiz_id=&user_id=&name=` (the socket is the join) |
| Answer an option | WS message `{ "type": "submit_answer", "payload": { questionId, answer } }` |
| Host Start / Next / End | `POST …/start`, `…/advance`, `…/end` |

Inbound WS messages drive the UI: `join_confirmed`, `user_joined`, `question`,
`score_update`, `leaderboard_update`, `quiz_ended`, and `error` (shown as a toast).
Each browser tab is a distinct player (`user_id` is a generated UUID). Answers are
sent over the **socket** so the score and leaderboard broadcast to everyone; this is
the same rule documented in the Postman demo guide.

## 5. Design notes

- **Embedded, no build:** `web/embed.go` (`//go:embed index.html`) exposes the bytes;
  `internal/handler/static.go` serves them at `GET /`. The route is a catch-all, so
  it serves the root and 404s unknown paths; the specific `/api/…`, `/ws`, `/metrics`
  routes win by `ServeMux` precedence. Verified by `internal/handler/static_test.go`.
- **Aesthetic:** "neon game-show" — one deep-ink ground, electric-cyan accent, hot
  magenta pop; bold display type, a countdown ring, and a leaderboard that bumps on
  score changes. The memorable anchors are the ring and the live board, not a custom
  webfont (the file stays self-contained and offline-safe, so it uses a strong system
  display stack rather than a network-loaded font).
- **Resilience:** the WebSocket wrapper auto-reconnects once; reconnecting with the
  same `user_id` restores the player's score from `join_confirmed` (state lives on
  the server).

## 6. Out of scope

Authentication (identity is the provided id), persistence, mobile-native clients, and
a question-authoring UI (the question set is built into the page). These mirror the
MVP scope in the PRD.

## 7. Verification

The Go static route is covered by `internal/handler/static_test.go` (RED→GREEN). The
client was verified against a running server: `GET /` serves the page; the create
payload is accepted; and a two-player run (create → both join → Start → answer over
WS → Next → End) produced the expected `score_update` / `leaderboard_update` /
`quiz_ended` broadcasts to both players. `make test`, `make test-race`, `make lint`,
and `make e2e` remain green.
