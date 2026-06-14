package domain

import "time"

type Participant struct {
	UserID       string
	SessionID    string
	DisplayName  string
	Score        int
	LastScoredAt time.Time
	JoinedAt     time.Time
	answered     map[string]bool
}

func NewParticipant(userID, sessionID, displayName string) *Participant {
	return &Participant{
		UserID:      userID,
		SessionID:   sessionID,
		DisplayName: displayName,
		JoinedAt:    time.Now(),
		answered:    make(map[string]bool),
	}
}

func (p *Participant) AddScore(points int, at time.Time) {
	p.Score += points
	p.LastScoredAt = at
}

func (p *Participant) HasAnswered(questionID string) bool {
	return p.answered[questionID]
}

func (p *Participant) RecordAnswer(questionID string) error {
	if p.answered[questionID] {
		return ErrDuplicateAnswer
	}
	p.answered[questionID] = true
	return nil
}
