package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewParticipant_InitializesWithZeroScore(t *testing.T) {
	p := NewParticipant("u1", "QUIZ-ABC", "Alice")
	if p.Score != 0 {
		t.Errorf("Score = %d, want 0", p.Score)
	}
	if p.UserID != "u1" || p.SessionID != "QUIZ-ABC" || p.DisplayName != "Alice" {
		t.Errorf("identity fields not set: %+v", p)
	}
	if p.HasAnswered("Q1") {
		t.Errorf("new participant should not have answered any question")
	}
}

func TestParticipant_AddScore_IncreasesScore(t *testing.T) {
	p := NewParticipant("u1", "QUIZ-ABC", "Alice")
	p.AddScore(10, time.Now())
	p.AddScore(5, time.Now())
	if p.Score != 15 {
		t.Errorf("Score = %d, want 15", p.Score)
	}
}

func TestParticipant_AddScore_StampsLastScoredAt(t *testing.T) {
	p := NewParticipant("u1", "QUIZ-ABC", "Alice")
	at := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	p.AddScore(10, at)
	if !p.LastScoredAt.Equal(at) {
		t.Errorf("LastScoredAt = %v, want %v", p.LastScoredAt, at)
	}
}

func TestParticipant_HasAnswered(t *testing.T) {
	p := NewParticipant("u1", "QUIZ-ABC", "Alice")
	if p.HasAnswered("Q1") {
		t.Fatalf("HasAnswered before recording = true, want false")
	}
	if err := p.RecordAnswer("Q1"); err != nil {
		t.Fatalf("RecordAnswer() = %v, want nil", err)
	}
	if !p.HasAnswered("Q1") {
		t.Errorf("HasAnswered after recording = false, want true")
	}
}

func TestParticipant_RecordAnswer_PreventsResubmission(t *testing.T) {
	p := NewParticipant("u1", "QUIZ-ABC", "Alice")
	if err := p.RecordAnswer("Q1"); err != nil {
		t.Fatalf("first RecordAnswer() = %v, want nil", err)
	}
	err := p.RecordAnswer("Q1")
	if !errors.Is(err, ErrDuplicateAnswer) {
		t.Errorf("second RecordAnswer() = %v, want ErrDuplicateAnswer", err)
	}
}
