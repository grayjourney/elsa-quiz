//go:build e2e

package e2e

import "encoding/json"

// These types are LOCAL copies of the server's wire contract. The harness never
// imports internal/... — the JSON contract itself is what is under test, so an
// accidental server-side rename surfaces here as a failing scenario.

// ---- REST: requests ----

type questionReq struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correctAnswer"`
	Order         int      `json:"order"`
}

type createSessionReq struct {
	EndPolicy        string        `json:"endPolicy"`
	TimeLimitSeconds int           `json:"timeLimitSeconds"`
	Questions        []questionReq `json:"questions"`
}

type joinReq struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

type submitReq struct {
	QuestionID string `json:"questionId"`
	Answer     string `json:"answer"`
}

// ---- REST: responses ----

type createSessionResp struct {
	QuizID           string `json:"quizId"`
	Status           string `json:"status"`
	EndPolicy        string `json:"endPolicy"`
	TimeLimitSeconds int    `json:"timeLimitSeconds"`
	QuestionCount    int    `json:"questionCount"`
}

type questionView struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Options []string `json:"options"`
	Order   int      `json:"order"`
}

type getSessionResp struct {
	QuizID           string        `json:"quizId"`
	Status           string        `json:"status"`
	EndPolicy        string        `json:"endPolicy"`
	ParticipantCount int           `json:"participantCount"`
	CurrentQuestion  *questionView `json:"currentQuestion,omitempty"`
}

type lbEntry struct {
	Rank        int    `json:"rank"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Score       int    `json:"score"`
}

type submitResp struct {
	IsCorrect     bool      `json:"isCorrect"`
	AwardedPoints int       `json:"awardedPoints"`
	NewScore      int       `json:"newScore"`
	Leaderboard   []lbEntry `json:"leaderboard"`
}

type leaderboardResp struct {
	Leaderboard []lbEntry `json:"leaderboard"`
}

type healthResp struct {
	Status         string `json:"status"`
	ActiveSessions int    `json:"activeSessions"`
	UptimeSeconds  int    `json:"uptimeSeconds"`
}

type errorResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---- WebSocket envelope + payloads ----

const (
	msgJoinConfirmed     = "join_confirmed"
	msgUserJoined        = "user_joined"
	msgQuestion          = "question"
	msgScoreUpdate       = "score_update"
	msgLeaderboardUpdate = "leaderboard_update"
	msgError             = "error"
	msgQuizEnded         = "quiz_ended"
	msgSubmitAnswer      = "submit_answer"
)

type wsEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type wsOutbound struct {
	Type    string    `json:"type"`
	Payload submitReq `json:"payload"`
}

type joinConfirmedPayload struct {
	UserID string `json:"userId"`
	QuizID string `json:"quizId"`
	Score  int    `json:"score"`
}

type userJoinedPayload struct {
	UserID           string `json:"userId"`
	DisplayName      string `json:"displayName"`
	ParticipantCount int    `json:"participantCount"`
}

type questionPayload struct {
	ID               string   `json:"id"`
	Text             string   `json:"text"`
	Options          []string `json:"options"`
	Order            int      `json:"order"`
	TimeLimitSeconds int      `json:"timeLimitSeconds,omitempty"`
}

type scoreUpdatePayload struct {
	UserID        string `json:"userId"`
	IsCorrect     bool   `json:"isCorrect"`
	AwardedPoints int    `json:"awardedPoints"`
	NewScore      int    `json:"newScore"`
}

// Note: leaderboard_update / quiz_ended / error envelopes are asserted by their
// `type` discriminator (see the msg* constants); their payload shapes are not
// decoded by any current scenario, so no structs are declared for them here.
