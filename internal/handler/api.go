package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gray/elsa-quiz/internal/domain"
	"github.com/gray/elsa-quiz/internal/service"
	"github.com/gray/elsa-quiz/internal/store"
)

type API struct {
	store       store.Store
	sessions    *service.SessionService
	scoring     *service.ScoringService
	leaderboard *service.LeaderboardService
	conns       *ConnectionManager
	upgrader    websocket.Upgrader
	metrics     *metrics
	sched       *scheduler
	now         func() time.Time
	startedAt   time.Time
}

func NewAPI(st store.Store, basePoints int) *API {
	a := &API{
		store:       st,
		sessions:    service.NewSessionService(st),
		scoring:     service.NewScoringService(st, basePoints),
		leaderboard: service.NewLeaderboardService(st),
		conns:       NewConnectionManager(),
		upgrader:    websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
		metrics:     newMetrics(st.ActiveSessionCount),
		now:         time.Now,
		startedAt:   time.Now(),
	}
	a.sched = newScheduler(a.advanceTimed)
	return a
}

// Shutdown stops any pending timed-quiz timers.
func (a *API) Shutdown() { a.sched.cancelAll() }

// afterAdvance broadcasts the result of an advance and manages the timed timer.
func (a *API) afterAdvance(s *domain.QuizSession, q domain.Question, ongoing bool) {
	if ongoing {
		a.conns.Broadcast(s.ID, Message(MsgQuestion, a.questionPayload(s, q)))
		if s.EndPolicy == domain.EndPolicyTimed {
			a.sched.schedule(s.ID, s.TimeLimit)
		}
		return
	}
	board, _ := a.leaderboard.GetLeaderboard(s.ID)
	a.conns.Broadcast(s.ID, Message(MsgQuizEnded, QuizEndedPayload{Leaderboard: leaderboardEntries(board)}))
	a.sched.cancel(s.ID)
}

// advanceTimed is the timer callback: advance the current question if it is
// still current (a host advance or early advance may have moved on first).
func (a *API) advanceTimed(quizID string) {
	a.tryAdvanceCurrent(quizID)
}

// maybeAdvanceEarly advances a timed question as soon as every connected
// participant has answered it, without waiting for the timer.
func (a *API) maybeAdvanceEarly(quizID string) {
	s, err := a.store.GetSession(quizID)
	if err != nil || s.EndPolicy != domain.EndPolicyTimed {
		return
	}
	if !s.AllAnsweredCurrent(a.conns.UserIDs(quizID)) {
		return
	}
	a.tryAdvanceCurrent(quizID)
}

func (a *API) tryAdvanceCurrent(quizID string) {
	s, err := a.store.GetSession(quizID)
	if err != nil || s.GetStatus() != domain.StatusActive || s.EndPolicy != domain.EndPolicyTimed {
		return
	}
	cur, ok := s.CurrentQuestion()
	if !ok {
		return
	}
	next, ongoing, advanced := s.AdvanceIfCurrent(cur.ID, a.now())
	if advanced {
		a.afterAdvance(s, next, ongoing)
	}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.Handle("GET /metrics", a.metrics.handler())
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("POST /api/sessions", a.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", a.handleGetSession)
	mux.HandleFunc("POST /api/sessions/{id}/participants", a.handleJoin)
	mux.HandleFunc("POST /api/sessions/{id}/start", a.handleStart)
	mux.HandleFunc("POST /api/sessions/{id}/advance", a.handleAdvance)
	mux.HandleFunc("POST /api/sessions/{id}/end", a.handleEnd)
	mux.HandleFunc("POST /api/sessions/{id}/answers", a.handleSubmitAnswer)
	mux.HandleFunc("GET /api/sessions/{id}/leaderboard", a.handleLeaderboard)
	mux.HandleFunc("GET /ws", a.handleWS)
	return mux
}

// ---- handlers ----

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"activeSessions": a.store.ActiveSessionCount(),
		"uptimeSeconds":  int(a.now().Sub(a.startedAt).Seconds()),
	})
}

type createSessionRequest struct {
	EndPolicy        string            `json:"endPolicy"`
	TimeLimitSeconds int               `json:"timeLimitSeconds"`
	Questions        []domain.Question `json:"questions"`
}

func (a *API) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if !decodeBody(w, r, &req) {
		return
	}
	for _, q := range req.Questions {
		if err := q.Validate(); err != nil {
			writeError(w, err)
			return
		}
	}
	policy := domain.EndPolicy(req.EndPolicy)
	limit := time.Duration(req.TimeLimitSeconds) * time.Second
	s, err := a.sessions.CreateSession(req.Questions, policy, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"quizId":           s.ID,
		"status":           s.Status,
		"endPolicy":        s.EndPolicy,
		"timeLimitSeconds": req.TimeLimitSeconds,
		"questionCount":    len(s.Questions),
	})
}

func (a *API) handleGetSession(w http.ResponseWriter, r *http.Request) {
	s, err := a.store.GetSession(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	resp := map[string]any{
		"quizId":           s.ID,
		"status":           s.GetStatus(),
		"endPolicy":        s.EndPolicy,
		"participantCount": len(s.Participants()),
	}
	if q, ok := s.CurrentQuestion(); ok {
		resp["currentQuestion"] = questionView(q)
	}
	writeJSON(w, http.StatusOK, resp)
}

type joinRequest struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

func (a *API) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req joinRequest
	if !decodeBody(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	p, err := a.sessions.JoinSession(id, req.UserID, req.DisplayName)
	if err != nil {
		writeError(w, err)
		return
	}
	s, _ := a.store.GetSession(id)
	writeJSON(w, http.StatusCreated, map[string]any{
		"userId":           p.UserID,
		"displayName":      p.DisplayName,
		"score":            p.Score,
		"participantCount": len(s.Participants()),
	})
}

func (a *API) handleStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.sessions.StartSession(id, a.now()); err != nil {
		writeError(w, err)
		return
	}
	s, _ := a.store.GetSession(id)
	resp := map[string]any{"status": s.GetStatus()}
	if q, ok := s.CurrentQuestion(); ok {
		resp["currentQuestion"] = questionView(q)
		a.conns.Broadcast(s.ID, Message(MsgQuestion, a.questionPayload(s, q)))
		if s.EndPolicy == domain.EndPolicyTimed {
			a.sched.schedule(s.ID, s.TimeLimit)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleAdvance(w http.ResponseWriter, r *http.Request) {
	s, err := a.store.GetSession(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	q, ongoing, err := s.AdvanceQuestion(a.now())
	if err != nil {
		writeError(w, err)
		return
	}
	a.afterAdvance(s, q, ongoing)
	resp := map[string]any{"ongoing": ongoing, "status": s.GetStatus()}
	if ongoing {
		resp["currentQuestion"] = questionView(q)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleEnd(w http.ResponseWriter, r *http.Request) {
	s, err := a.store.GetSession(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if s.GetStatus() == domain.StatusCompleted {
		writeError(w, domain.ErrQuizEnded)
		return
	}
	s.Complete()
	a.sched.cancel(s.ID)
	board, _ := a.leaderboard.GetLeaderboard(s.ID)
	a.conns.Broadcast(s.ID, Message(MsgQuizEnded, QuizEndedPayload{Leaderboard: leaderboardEntries(board)}))
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      s.GetStatus(),
		"leaderboard": leaderboardView(board),
	})
}

type submitRequest struct {
	QuestionID string `json:"questionId"`
	Answer     string `json:"answer"`
}

func (a *API) handleSubmitAnswer(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if !decodeBody(w, r, &req) {
		return
	}
	userID := r.Header.Get("X-User-ID")
	start := time.Now()
	res, board, err := a.scoring.SubmitAnswer(r.PathValue("id"), userID, req.QuestionID, req.Answer, a.now())
	if err != nil {
		a.metrics.record(start, "error")
		writeError(w, err)
		return
	}
	a.metrics.record(start, outcomeLabel(res.IsCorrect))
	writeJSON(w, http.StatusOK, map[string]any{
		"isCorrect":     res.IsCorrect,
		"awardedPoints": res.AwardedPoints,
		"newScore":      res.NewScore,
		"leaderboard":   leaderboardView(board),
	})
	a.maybeAdvanceEarly(r.PathValue("id"))
}

func (a *API) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	board, err := a.leaderboard.GetLeaderboard(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"leaderboard": leaderboardView(board)})
}

// ---- views ----

func questionView(q domain.Question) map[string]any {
	return map[string]any{"id": q.ID, "text": q.Text, "options": q.Options, "order": q.Order}
}

func leaderboardView(entries []domain.LeaderboardEntry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{"rank": e.Rank, "userId": e.UserID, "displayName": e.DisplayName, "score": e.Score})
	}
	return out
}

// ---- helpers ----

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "invalid_body", "message": "Request body is not valid JSON"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func errorsAsDomain(err error) (*domain.Error, bool) {
	return errors.AsType[*domain.Error](err)
}

func writeError(w http.ResponseWriter, err error) {
	if de, ok := errorsAsDomain(err); ok {
		writeJSON(w, statusFor(de), map[string]any{"code": de.Code, "message": de.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"code": "internal", "message": "Internal server error"})
}

func statusFor(e *domain.Error) int {
	switch e.Code {
	case "session_not_found", "question_not_found", "participant_not_found":
		return http.StatusNotFound
	case "quiz_id_required", "answer_empty", "invalid_option", "invalid_time_limit", "invalid_question":
		return http.StatusBadRequest
	case "duplicate_answer", "session_ended", "quiz_ended", "time_up", "invalid_session_state", "session_exists":
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
