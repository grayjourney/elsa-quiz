package handler

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gray/elsa-quiz/internal/domain"
)

type wsConn struct{ conn *websocket.Conn }

func (c *wsConn) WriteMessage(data []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, data)
}
func (c *wsConn) Close() error { return c.conn.Close() }

func (a *API) handleWS(w http.ResponseWriter, r *http.Request) {
	quizID := r.URL.Query().Get("quiz_id")
	userID := r.URL.Query().Get("user_id")
	name := r.URL.Query().Get("name")
	if name == "" {
		name = userID
	}

	// Validate + join before upgrading, so failures return a clean HTTP error.
	p, err := a.sessions.JoinSession(quizID, userID, name)
	if err != nil {
		writeError(w, err)
		return
	}
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	a.conns.Register(quizID, userID, &wsConn{conn})
	a.metrics.connectedUsers.Inc()
	defer func() {
		a.conns.Unregister(quizID, userID)
		a.metrics.connectedUsers.Dec()
	}()

	a.conns.SendTo(quizID, userID, Message(MsgJoinConfirmed, JoinConfirmedPayload{UserID: userID, QuizID: quizID, Score: p.Score}))

	s, _ := a.store.GetSession(quizID)
	a.conns.BroadcastExcept(quizID, userID, Message(MsgUserJoined, UserJoinedPayload{
		UserID: userID, DisplayName: name, ParticipantCount: len(s.Participants()),
	}))
	if q, ok := s.CurrentQuestion(); ok {
		a.conns.SendTo(quizID, userID, Message(MsgQuestion, a.questionPayload(s, q)))
	}

	a.readLoop(quizID, userID, conn)
}

func (a *API) readLoop(quizID, userID string, conn *websocket.Conn) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return // disconnect — score/state is preserved server-side
		}
		env, err := ParseClientMessage(data)
		if err != nil {
			a.conns.SendTo(quizID, userID, Message(MsgError, ErrorPayload{Code: "invalid_message", Message: "Message is not valid JSON"}))
			continue
		}
		if env.Type == MsgSubmitAnswer {
			sa, err := env.AsSubmitAnswer()
			if err != nil {
				a.conns.SendTo(quizID, userID, Message(MsgError, ErrorPayload{Code: "invalid_message", Message: "Invalid submit_answer payload"}))
				continue
			}
			a.processAnswer(quizID, userID, sa)
		}
	}
}

func (a *API) processAnswer(quizID, userID string, sa SubmitAnswerPayload) {
	start := time.Now()
	res, board, err := a.scoring.SubmitAnswer(quizID, userID, sa.QuestionID, sa.Answer, a.now())
	if err != nil {
		a.metrics.record(start, "error")
		a.conns.SendTo(quizID, userID, Message(MsgError, errorPayload(err)))
		return
	}
	a.metrics.record(start, outcomeLabel(res.IsCorrect))
	a.conns.Broadcast(quizID, Message(MsgScoreUpdate, ScoreUpdatePayload{
		UserID: userID, IsCorrect: res.IsCorrect, AwardedPoints: res.AwardedPoints, NewScore: res.NewScore,
	}))
	a.conns.Broadcast(quizID, Message(MsgLeaderboardUpdate, LeaderboardUpdatePayload{Entries: leaderboardEntries(board)}))
	a.maybeAdvanceEarly(quizID)
}

func (a *API) questionPayload(s *domain.QuizSession, q domain.Question) QuestionPayload {
	return QuestionPayload{
		ID: q.ID, Text: q.Text, Options: q.Options, Order: q.Order,
		TimeLimitSeconds: int(s.TimeLimit.Seconds()),
	}
}

func errorPayload(err error) ErrorPayload {
	if de, ok := errorsAsDomain(err); ok {
		return ErrorPayload{Code: de.Code, Message: de.Message}
	}
	return ErrorPayload{Code: "internal", Message: "Internal server error"}
}

func outcomeLabel(correct bool) string {
	if correct {
		return "correct"
	}
	return "incorrect"
}

func leaderboardEntries(entries []domain.LeaderboardEntry) []LeaderboardEntryView {
	out := make([]LeaderboardEntryView, 0, len(entries))
	for _, e := range entries {
		out = append(out, LeaderboardEntryView{Rank: e.Rank, UserID: e.UserID, DisplayName: e.DisplayName, Score: e.Score})
	}
	return out
}
