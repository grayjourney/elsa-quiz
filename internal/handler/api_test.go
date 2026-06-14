package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gray/elsa-quiz/internal/store"
)

const basePoints = 10

func newTestAPI(t *testing.T) *API {
	t.Helper()
	return NewAPI(store.NewMemoryStore(), basePoints)
}

func do(t *testing.T, h http.Handler, method, path, userID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return m
}

func manualSessionBody() map[string]any {
	return map[string]any{
		"endPolicy":        "manual",
		"timeLimitSeconds": 0,
		"questions": []map[string]any{
			{"id": "Q1", "text": "past tense of go", "options": []string{"goed", "went", "gone", "going"}, "correctAnswer": "went", "order": 1},
			{"id": "Q2", "text": "plural of mouse", "options": []string{"mouses", "mice"}, "correctAnswer": "mice", "order": 2},
		},
	}
}

func createSession(t *testing.T, h http.Handler, body map[string]any) string {
	t.Helper()
	rec := do(t, h, http.MethodPost, "/api/sessions", "", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create session: code %d, body %s", rec.Code, rec.Body.String())
	}
	return decode(t, rec)["quizId"].(string)
}

// ---- Health & Observability ----

func TestAPI_Health(t *testing.T) {
	rec := do(t, newTestAPI(t).Handler(), http.MethodGet, "/api/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if decode(t, rec)["status"] != "ok" {
		t.Errorf("status field = %v, want ok", decode(t, rec)["status"])
	}
}

func TestAPI_Metrics(t *testing.T) {
	rec := do(t, newTestAPI(t).Handler(), http.MethodGet, "/metrics", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "quiz_active_sessions") {
		t.Errorf("metrics body missing quiz_active_sessions:\n%s", rec.Body.String())
	}
}

// ---- Session Management ----

func TestAPI_CreateSession_Manual(t *testing.T) {
	rec := do(t, newTestAPI(t).Handler(), http.MethodPost, "/api/sessions", "", manualSessionBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String())
	}
	b := decode(t, rec)
	if id, ok := b["quizId"].(string); !ok || len(id) < 6 || id[:5] != "QUIZ-" {
		t.Errorf("quizId = %v, want QUIZ- prefix", b["quizId"])
	}
	if b["status"] != "waiting" {
		t.Errorf("status = %v, want waiting", b["status"])
	}
}

func TestAPI_CreateSession_TimedInvalidLimit(t *testing.T) {
	body := map[string]any{"endPolicy": "timed", "timeLimitSeconds": 0,
		"questions": []map[string]any{{"id": "Q1", "text": "t", "options": []string{"a", "b"}, "correctAnswer": "a", "order": 1}}}
	rec := do(t, newTestAPI(t).Handler(), http.MethodPost, "/api/sessions", "", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
	if decode(t, rec)["code"] != "invalid_time_limit" {
		t.Errorf("code = %v, want invalid_time_limit", decode(t, rec)["code"])
	}
}

func TestAPI_GetSession_NotFound(t *testing.T) {
	rec := do(t, newTestAPI(t).Handler(), http.MethodGet, "/api/sessions/QUIZ-999", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
	if decode(t, rec)["code"] != "session_not_found" {
		t.Errorf("code = %v, want session_not_found", decode(t, rec)["code"])
	}
}

// ---- Join ----

func TestAPI_Join_Valid(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := createSession(t, h, manualSessionBody())
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/participants", "", map[string]any{"userId": "alice", "displayName": "Alice"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String())
	}
	b := decode(t, rec)
	if b["score"].(float64) != 0 {
		t.Errorf("score = %v, want 0", b["score"])
	}
}

func TestAPI_Join_NotFound(t *testing.T) {
	rec := do(t, newTestAPI(t).Handler(), http.MethodPost, "/api/sessions/QUIZ-999/participants", "", map[string]any{"userId": "alice", "displayName": "Alice"})
	if rec.Code != http.StatusNotFound || decode(t, rec)["code"] != "session_not_found" {
		t.Errorf("code = %d / %v, want 404 / session_not_found", rec.Code, decode(t, rec)["code"])
	}
}

func TestAPI_Join_Completed(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := createSession(t, h, manualSessionBody())
	if rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/end", "host", nil); rec.Code != http.StatusOK {
		t.Fatalf("end: code %d", rec.Code)
	}
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/participants", "", map[string]any{"userId": "alice", "displayName": "Alice"})
	if rec.Code != http.StatusConflict || decode(t, rec)["code"] != "session_ended" {
		t.Errorf("code = %d / %v, want 409 / session_ended", rec.Code, decode(t, rec)["code"])
	}
}

// ---- Lifecycle ----

func TestAPI_Start_BroadcastsFirstQuestion(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := createSession(t, h, manualSessionBody())
	_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/participants", "", map[string]any{"userId": "alice", "displayName": "Alice"})
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String())
	}
	b := decode(t, rec)
	if b["status"] != "active" {
		t.Errorf("status = %v, want active", b["status"])
	}
	q := b["currentQuestion"].(map[string]any)
	if q["id"] != "Q1" {
		t.Errorf("currentQuestion.id = %v, want Q1", q["id"])
	}
}

func TestAPI_Advance_ThenComplete(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := createSession(t, h, manualSessionBody())
	_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)

	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/advance", "host", nil)
	b := decode(t, rec)
	if b["ongoing"] != true || b["currentQuestion"].(map[string]any)["id"] != "Q2" {
		t.Errorf("advance = %v, want ongoing Q2", b)
	}
	rec = do(t, h, http.MethodPost, "/api/sessions/"+id+"/advance", "host", nil)
	if decode(t, rec)["ongoing"] != false || decode(t, rec)["status"] != "completed" {
		t.Errorf("advance past last = %v, want completed", rec.Body.String())
	}
}

func TestAPI_End_AndEndAgain(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := createSession(t, h, manualSessionBody())
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/end", "host", nil)
	if rec.Code != http.StatusOK || decode(t, rec)["status"] != "completed" {
		t.Fatalf("end: code %d body %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodPost, "/api/sessions/"+id+"/end", "host", nil)
	if rec.Code != http.StatusConflict || decode(t, rec)["code"] != "quiz_ended" {
		t.Errorf("end again = %d / %v, want 409 / quiz_ended", rec.Code, decode(t, rec)["code"])
	}
}

// ---- Answers ----

func startedSessionWith(t *testing.T, h http.Handler, users ...string) string {
	t.Helper()
	id := createSession(t, h, manualSessionBody())
	for _, u := range users {
		_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/participants", "", map[string]any{"userId": u, "displayName": u})
	}
	_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)
	return id
}

func TestAPI_SubmitCorrect(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := startedSessionWith(t, h, "alice", "bob")
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", "alice", map[string]any{"questionId": "Q1", "answer": "went"})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body %s", rec.Code, rec.Body.String())
	}
	b := decode(t, rec)
	if b["isCorrect"] != true || b["awardedPoints"].(float64) != 10 || b["newScore"].(float64) != 10 {
		t.Errorf("result = %v, want correct +10 total 10", b)
	}
	if _, ok := b["leaderboard"].([]any); !ok {
		t.Errorf("missing leaderboard array")
	}
}

func TestAPI_SubmitIncorrect(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := startedSessionWith(t, h, "alice")
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", "alice", map[string]any{"questionId": "Q1", "answer": "goed"})
	b := decode(t, rec)
	if rec.Code != http.StatusOK || b["isCorrect"] != false || b["newScore"].(float64) != 0 {
		t.Errorf("result = %d %v, want 200 incorrect 0", rec.Code, b)
	}
}

func TestAPI_SubmitErrors(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, h http.Handler, id string)
		user     string
		payload  map[string]any
		wantCode int
		wantErr  string
	}{
		{"duplicate", func(t *testing.T, h http.Handler, id string) {
			_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", "alice", map[string]any{"questionId": "Q1", "answer": "went"})
		}, "alice", map[string]any{"questionId": "Q1", "answer": "gone"}, http.StatusConflict, "duplicate_answer"},
		{"unknown question", nil, "alice", map[string]any{"questionId": "Q-NOPE", "answer": "went"}, http.StatusNotFound, "question_not_found"},
		{"empty answer", nil, "alice", map[string]any{"questionId": "Q1", "answer": ""}, http.StatusBadRequest, "answer_empty"},
		{"invalid option", nil, "alice", map[string]any{"questionId": "Q1", "answer": "walked"}, http.StatusBadRequest, "invalid_option"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestAPI(t).Handler()
			id := startedSessionWith(t, h, "alice")
			if tt.setup != nil {
				tt.setup(t, h, id)
			}
			rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", tt.user, tt.payload)
			if rec.Code != tt.wantCode || decode(t, rec)["code"] != tt.wantErr {
				t.Errorf("got %d / %v, want %d / %s", rec.Code, decode(t, rec)["code"], tt.wantCode, tt.wantErr)
			}
		})
	}
}

func TestAPI_Submit_MissingUserHeader(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := startedSessionWith(t, h, "alice")
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", "", map[string]any{"questionId": "Q1", "answer": "went"})
	if rec.Code != http.StatusNotFound || decode(t, rec)["code"] != "participant_not_found" {
		t.Errorf("got %d / %v, want 404 / participant_not_found (session exists, user did not join)", rec.Code, decode(t, rec)["code"])
	}
}

func TestAPI_SubmitAfterEnded(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := startedSessionWith(t, h, "alice")
	_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/end", "host", nil)
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", "alice", map[string]any{"questionId": "Q1", "answer": "went"})
	if rec.Code != http.StatusConflict || decode(t, rec)["code"] != "quiz_ended" {
		t.Errorf("got %d / %v, want 409 / quiz_ended", rec.Code, decode(t, rec)["code"])
	}
}

func TestAPI_SubmitTimedTimeUp(t *testing.T) {
	api := newTestAPI(t)
	t0 := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	api.now = func() time.Time { return t0 }
	h := api.Handler()
	body := map[string]any{"endPolicy": "timed", "timeLimitSeconds": 30,
		"questions": []map[string]any{{"id": "Q1", "text": "t", "options": []string{"a", "went"}, "correctAnswer": "went", "order": 1}}}
	id := createSession(t, h, body)
	_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/participants", "", map[string]any{"userId": "alice", "displayName": "Alice"})
	_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/start", "host", nil)

	api.now = func() time.Time { return t0.Add(31 * time.Second) }
	rec := do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", "alice", map[string]any{"questionId": "Q1", "answer": "went"})
	if rec.Code != http.StatusConflict || decode(t, rec)["code"] != "time_up" {
		t.Errorf("got %d / %v, want 409 / time_up", rec.Code, decode(t, rec)["code"])
	}
}

// ---- Leaderboard ----

func TestAPI_Leaderboard_Ranked(t *testing.T) {
	h := newTestAPI(t).Handler()
	id := startedSessionWith(t, h, "alice", "bob")
	_ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/answers", "alice", map[string]any{"questionId": "Q1", "answer": "went"})

	rec := do(t, h, http.MethodGet, "/api/sessions/"+id+"/leaderboard", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	entries := decode(t, rec)["leaderboard"].([]any)
	top := entries[0].(map[string]any)
	if top["userId"] != "alice" || top["rank"].(float64) != 1 || top["score"].(float64) != 10 {
		t.Errorf("top entry = %v, want alice rank 1 score 10", top)
	}
}
