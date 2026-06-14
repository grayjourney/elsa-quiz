package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gray/elsa-quiz/internal/store"
)

// TestE2E_Scenarios walks the docs/02-test-cases.md scenarios end-to-end against
// a live server with real WebSocket clients and the REST control API.
func TestE2E_Scenarios(t *testing.T) {
	api := NewAPI(store.NewMemoryStore(), basePoints)
	h := api.Handler()
	srv := httptest.NewServer(h)
	defer srv.Close()

	createSession := func(t *testing.T) string {
		t.Helper()
		body := map[string]any{"endPolicy": "manual", "timeLimitSeconds": 0,
			"questions": []map[string]any{{"id": "Q1", "text": "past tense of go", "options": []string{"goed", "went"}, "correctAnswer": "went", "order": 1}}}
		rec := do(t, h, http.MethodPost, "/api/sessions", "", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: %d", rec.Code)
		}
		return decode(t, rec)["quizId"].(string)
	}

	t.Run("Quiz Session Management: multi-user join notifies existing participants", func(t *testing.T) {
		id := createSession(t)
		alice := wsDial(t, srv.URL, id, "alice", "Alice")
		defer func() { _ = alice.Close() }()
		expectType(t, alice, MsgJoinConfirmed)

		bob := wsDial(t, srv.URL, id, "bob", "Bob")
		defer func() { _ = bob.Close() }()
		expectType(t, bob, MsgJoinConfirmed)

		uj := expectType(t, alice, MsgUserJoined)
		var p UserJoinedPayload
		_ = json.Unmarshal(uj.Payload, &p)
		if p.UserID != "bob" || p.ParticipantCount != 2 {
			t.Errorf("user_joined = %+v, want bob/2", p)
		}
	})

	t.Run("Score Updates: correct answer broadcasts to all participants under 500ms", func(t *testing.T) {
		id := createSession(t)
		alice := wsDial(t, srv.URL, id, "alice", "Alice")
		defer func() { _ = alice.Close() }()
		bob := wsDial(t, srv.URL, id, "bob", "Bob")
		defer func() { _ = bob.Close() }()
		expectType(t, alice, MsgJoinConfirmed)
		expectType(t, bob, MsgJoinConfirmed)
		expectType(t, alice, MsgUserJoined)

		do(t, h, http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)
		expectType(t, alice, MsgQuestion)
		expectType(t, bob, MsgQuestion)

		start := time.Now()
		_ = alice.WriteMessage(websocket.TextMessage, Message(MsgSubmitAnswer, SubmitAnswerPayload{QuestionID: "Q1", Answer: "went"}))

		su := expectType(t, alice, MsgScoreUpdate)
		var sp ScoreUpdatePayload
		_ = json.Unmarshal(su.Payload, &sp)
		if sp.NewScore != 10 || !sp.IsCorrect {
			t.Errorf("alice score_update = %+v, want 10/correct", sp)
		}
		expectType(t, alice, MsgLeaderboardUpdate)
		expectType(t, bob, MsgScoreUpdate) // bob (other connection) also notified
		expectType(t, bob, MsgLeaderboardUpdate)
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Errorf("broadcast latency %v exceeds 500ms", elapsed)
		}
	})

	t.Run("Reliability: reconnect resumes with the preserved score", func(t *testing.T) {
		id := createSession(t)
		alice := wsDial(t, srv.URL, id, "alice", "Alice")
		expectType(t, alice, MsgJoinConfirmed)
		do(t, h, http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)
		expectType(t, alice, MsgQuestion)
		_ = alice.WriteMessage(websocket.TextMessage, Message(MsgSubmitAnswer, SubmitAnswerPayload{QuestionID: "Q1", Answer: "went"}))
		expectType(t, alice, MsgScoreUpdate)
		expectType(t, alice, MsgLeaderboardUpdate)

		// Drop the connection, then reconnect with the same user_id.
		_ = alice.Close()
		time.Sleep(50 * time.Millisecond)

		alice2 := wsDial(t, srv.URL, id, "alice", "Alice")
		defer func() { _ = alice2.Close() }()
		jc := expectType(t, alice2, MsgJoinConfirmed)
		var p JoinConfirmedPayload
		_ = json.Unmarshal(jc.Payload, &p)
		if p.Score != 10 {
			t.Errorf("reconnect join_confirmed score = %d, want 10 (state preserved across disconnect)", p.Score)
		}
	})

	t.Run("Session Lifecycle: ending the quiz broadcasts quiz_ended", func(t *testing.T) {
		id := createSession(t)
		alice := wsDial(t, srv.URL, id, "alice", "Alice")
		defer func() { _ = alice.Close() }()
		expectType(t, alice, MsgJoinConfirmed)
		do(t, h, http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)
		expectType(t, alice, MsgQuestion)

		do(t, h, http.MethodPost, "/api/sessions/"+id+"/end", "host", nil)
		expectType(t, alice, MsgQuizEnded)
	})

	t.Run("Concurrency: 20 users submit simultaneously, no scores lost", func(t *testing.T) {
		id := createSession(t)
		const n = 20
		for i := range n {
			u := fmt.Sprintf("u%d", i)
			do(t, h, http.MethodPost, "/api/sessions/"+id+"/participants", "", map[string]any{"userId": u, "displayName": u})
		}
		do(t, h, http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)

		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", fmt.Sprintf("u%d", i), map[string]any{"questionId": "Q1", "answer": "went"})
			}(i)
		}
		wg.Wait()

		rec := do(t, h, http.MethodGet, "/api/sessions/"+id+"/leaderboard", "", nil)
		entries := decode(t, rec)["leaderboard"].([]any)
		total := 0
		for _, e := range entries {
			total += int(e.(map[string]any)["score"].(float64))
		}
		if total != n*10 {
			t.Errorf("total score = %d, want %d (no lost scores under concurrency)", total, n*10)
		}
	})
}
