package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gray/elsa-quiz/internal/domain"
	"github.com/gray/elsa-quiz/internal/store"
)

func wsDial(t *testing.T, srvURL, quizID, userID, name string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(srvURL, "http") + "/ws?quiz_id=" + quizID + "&user_id=" + userID + "&name=" + name
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", userID, err)
	}
	return c
}

func expectType(t *testing.T, c *websocket.Conn, want string) Envelope {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read (want %s): %v", want, err)
	}
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if e.Type != want {
		t.Fatalf("message type = %q, want %q (body %s)", e.Type, want, data)
	}
	return e
}

func TestWS_RealtimeFlow(t *testing.T) {
	api := NewAPI(store.NewMemoryStore(), basePoints)
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	s, err := api.sessions.CreateSession([]domain.Question{
		{ID: "Q1", Text: "past tense of go", Options: []string{"goed", "went"}, CorrectAnswer: "went", Order: 1},
	}, domain.EndPolicyManual, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := s.ID

	// Alice connects → join_confirmed
	alice := wsDial(t, srv.URL, id, "alice", "Alice")
	defer func() { _ = alice.Close() }()
	expectType(t, alice, MsgJoinConfirmed)

	// Bob connects → his join_confirmed; Alice is notified via user_joined
	bob := wsDial(t, srv.URL, id, "bob", "Bob")
	defer func() { _ = bob.Close() }()
	expectType(t, bob, MsgJoinConfirmed)
	joined := expectType(t, alice, MsgUserJoined)
	var uj UserJoinedPayload
	_ = json.Unmarshal(joined.Payload, &uj)
	if uj.UserID != "bob" || uj.ParticipantCount != 2 {
		t.Errorf("user_joined = %+v, want bob/2", uj)
	}

	// Host starts the quiz (REST) → both clients receive the question
	rec := do(t, api.Handler(), "POST", "/api/sessions/"+id+"/start", "host", nil)
	if rec.Code != 200 {
		t.Fatalf("start: %d", rec.Code)
	}
	expectType(t, alice, MsgQuestion)
	expectType(t, bob, MsgQuestion)

	// Alice submits a correct answer over WS → both get score_update + leaderboard_update
	if err := alice.WriteMessage(websocket.TextMessage,
		Message(MsgSubmitAnswer, SubmitAnswerPayload{QuestionID: "Q1", Answer: "went"})); err != nil {
		t.Fatalf("write submit: %v", err)
	}

	su := expectType(t, alice, MsgScoreUpdate)
	var sup ScoreUpdatePayload
	_ = json.Unmarshal(su.Payload, &sup)
	if sup.UserID != "alice" || sup.NewScore != 10 || !sup.IsCorrect {
		t.Errorf("score_update = %+v, want alice/10/correct", sup)
	}
	expectType(t, alice, MsgLeaderboardUpdate)

	// Bob (a different connection) receives the same broadcasts
	expectType(t, bob, MsgScoreUpdate)
	lb := expectType(t, bob, MsgLeaderboardUpdate)
	var lbp LeaderboardUpdatePayload
	_ = json.Unmarshal(lb.Payload, &lbp)
	if len(lbp.Entries) != 2 || lbp.Entries[0].UserID != "alice" || lbp.Entries[0].Score != 10 {
		t.Errorf("leaderboard = %+v, want alice top with 10", lbp.Entries)
	}
}
