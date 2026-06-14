package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gray/elsa-quiz/internal/domain"
	"github.com/gray/elsa-quiz/internal/store"
)

func timedSession(t *testing.T, api *API, limit time.Duration) string {
	t.Helper()
	s, err := api.sessions.CreateSession([]domain.Question{
		{ID: "Q1", Text: "past tense of go", Options: []string{"goed", "went"}, CorrectAnswer: "went", Order: 1},
		{ID: "Q2", Text: "plural of mouse", Options: []string{"mouses", "mice"}, CorrectAnswer: "mice", Order: 2},
	}, domain.EndPolicyTimed, limit)
	if err != nil {
		t.Fatalf("create timed session: %v", err)
	}
	return s.ID
}

func questionID(t *testing.T, env Envelope) string {
	t.Helper()
	var p QuestionPayload
	_ = json.Unmarshal(env.Payload, &p)
	return p.ID
}

// Timer expires with no answer → question auto-advances, then the quiz auto-completes.
func TestWS_Timed_AutoAdvanceOnExpiry(t *testing.T) {
	api := NewAPI(store.NewMemoryStore(), basePoints)
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	id := timedSession(t, api, 80*time.Millisecond)
	alice := wsDial(t, srv.URL, id, "alice", "Alice")
	defer func() { _ = alice.Close() }()
	expectType(t, alice, MsgJoinConfirmed)

	do(t, api.Handler(), http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)
	if got := questionID(t, expectType(t, alice, MsgQuestion)); got != "Q1" {
		t.Fatalf("first question = %q, want Q1", got)
	}
	// No answer — the timer should advance to Q2 on its own.
	if got := questionID(t, expectType(t, alice, MsgQuestion)); got != "Q2" {
		t.Fatalf("auto-advanced question = %q, want Q2", got)
	}
	// After Q2's limit, the quiz auto-completes.
	expectType(t, alice, MsgQuizEnded)
}

// All connected participants answer before the limit → advance early (no wait).
func TestWS_Timed_AdvanceEarlyWhenAllAnswered(t *testing.T) {
	api := NewAPI(store.NewMemoryStore(), basePoints)
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	// Long limit so a pass can only come from early advance, not the timer.
	id := timedSession(t, api, 10*time.Second)
	alice := wsDial(t, srv.URL, id, "alice", "Alice")
	defer func() { _ = alice.Close() }()
	expectType(t, alice, MsgJoinConfirmed)

	do(t, api.Handler(), http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)
	expectType(t, alice, MsgQuestion) // Q1

	_ = alice.WriteMessage(websocket.TextMessage, Message(MsgSubmitAnswer, SubmitAnswerPayload{QuestionID: "Q1", Answer: "went"}))
	expectType(t, alice, MsgScoreUpdate)
	expectType(t, alice, MsgLeaderboardUpdate)
	if got := questionID(t, expectType(t, alice, MsgQuestion)); got != "Q2" {
		t.Fatalf("early-advanced question = %q, want Q2 (should not wait for the 10s timer)", got)
	}
}
