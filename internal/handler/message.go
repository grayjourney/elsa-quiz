package handler

import "encoding/json"

const (
	// client -> server
	MsgSubmitAnswer = "submit_answer"
	// server -> client
	MsgJoinConfirmed     = "join_confirmed"
	MsgUserJoined        = "user_joined"
	MsgQuestion          = "question"
	MsgScoreUpdate       = "score_update"
	MsgLeaderboardUpdate = "leaderboard_update"
	MsgError             = "error"
	MsgQuizEnded         = "quiz_ended"
)

type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type SubmitAnswerPayload struct {
	QuestionID string `json:"questionId"`
	Answer     string `json:"answer"`
}

type JoinConfirmedPayload struct {
	UserID string `json:"userId"`
	QuizID string `json:"quizId"`
	Score  int    `json:"score"`
}

type UserJoinedPayload struct {
	UserID           string `json:"userId"`
	DisplayName      string `json:"displayName"`
	ParticipantCount int    `json:"participantCount"`
}

type QuestionPayload struct {
	ID               string   `json:"id"`
	Text             string   `json:"text"`
	Options          []string `json:"options"`
	Order            int      `json:"order"`
	TimeLimitSeconds int      `json:"timeLimitSeconds,omitempty"`
}

type ScoreUpdatePayload struct {
	UserID        string `json:"userId"`
	IsCorrect     bool   `json:"isCorrect"`
	AwardedPoints int    `json:"awardedPoints"`
	NewScore      int    `json:"newScore"`
}

type LeaderboardEntryView struct {
	Rank        int    `json:"rank"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Score       int    `json:"score"`
}

type LeaderboardUpdatePayload struct {
	Entries []LeaderboardEntryView `json:"entries"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type QuizEndedPayload struct {
	Leaderboard []LeaderboardEntryView `json:"leaderboard"`
}

func Message(msgType string, payload any) []byte {
	p, err := json.Marshal(payload)
	if err != nil {
		p, _ = json.Marshal(ErrorPayload{Code: "encode_error", Message: "failed to encode message"})
		msgType = MsgError
	}
	raw, _ := json.Marshal(Envelope{Type: msgType, Payload: p})
	return raw
}

func ParseClientMessage(data []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

func (e Envelope) AsSubmitAnswer() (SubmitAnswerPayload, error) {
	var p SubmitAnswerPayload
	err := json.Unmarshal(e.Payload, &p)
	return p, err
}
