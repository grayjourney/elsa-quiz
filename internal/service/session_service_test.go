package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gray/elsa-quiz/internal/domain"
	"github.com/gray/elsa-quiz/internal/store"
)

func sampleQuestions() []domain.Question {
	return []domain.Question{
		{ID: "Q1", Text: "past tense of go", Options: []string{"goed", "went"}, CorrectAnswer: "went", Order: 1},
	}
}

func newSessionService(t *testing.T) *SessionService {
	t.Helper()
	return NewSessionService(store.NewMemoryStore())
}

func TestSessionService_CreateSession_ReturnsStoredWaitingSession(t *testing.T) {
	svc := newSessionService(t)
	s, err := svc.CreateSession(sampleQuestions(), domain.EndPolicyManual, 0)
	if err != nil {
		t.Fatalf("CreateSession() = %v, want nil", err)
	}
	if !strings.HasPrefix(s.ID, "QUIZ-") {
		t.Errorf("session ID = %q, want QUIZ- prefix", s.ID)
	}
	if s.Status != domain.StatusWaiting {
		t.Errorf("Status = %q, want waiting", s.Status)
	}
	if _, err := svc.store.GetSession(s.ID); err != nil {
		t.Errorf("session not persisted: GetSession() = %v", err)
	}
}

func TestSessionService_CreateSession_TimedInvalidLimit(t *testing.T) {
	svc := newSessionService(t)
	if _, err := svc.CreateSession(sampleQuestions(), domain.EndPolicyTimed, 0); !errors.Is(err, domain.ErrInvalidTimeLimit) {
		t.Errorf("CreateSession(timed, 0) = %v, want ErrInvalidTimeLimit", err)
	}
}

func TestSessionService_JoinSession_AddsParticipant(t *testing.T) {
	svc := newSessionService(t)
	s, _ := svc.CreateSession(sampleQuestions(), domain.EndPolicyManual, 0)

	p, err := svc.JoinSession(s.ID, "u1", "Alice")
	if err != nil {
		t.Fatalf("JoinSession() = %v, want nil", err)
	}
	if p.DisplayName != "Alice" || p.Score != 0 {
		t.Errorf("participant = %+v, want Alice with score 0", p)
	}
	if got := len(s.Participants()); got != 1 {
		t.Errorf("participant count = %d, want 1", got)
	}
}

func TestSessionService_JoinSession_Errors(t *testing.T) {
	svc := newSessionService(t)
	s, _ := svc.CreateSession(sampleQuestions(), domain.EndPolicyManual, 0)

	t.Run("empty quiz id", func(t *testing.T) {
		if _, err := svc.JoinSession("", "u1", "Alice"); !errors.Is(err, domain.ErrQuizIDRequired) {
			t.Errorf("err = %v, want ErrQuizIDRequired", err)
		}
	})
	t.Run("non-existent", func(t *testing.T) {
		if _, err := svc.JoinSession("QUIZ-NOPE", "u1", "Alice"); !errors.Is(err, domain.ErrSessionNotFound) {
			t.Errorf("err = %v, want ErrSessionNotFound", err)
		}
	})
	t.Run("completed", func(t *testing.T) {
		s.Complete()
		if _, err := svc.JoinSession(s.ID, "u1", "Alice"); !errors.Is(err, domain.ErrSessionEnded) {
			t.Errorf("err = %v, want ErrSessionEnded", err)
		}
	})
}

func TestSessionService_StartSession(t *testing.T) {
	svc := newSessionService(t)
	s, _ := svc.CreateSession(sampleQuestions(), domain.EndPolicyManual, 0)
	if err := svc.StartSession(s.ID, time.Now()); err != nil {
		t.Fatalf("StartSession() = %v, want nil", err)
	}
	if s.GetStatus() != domain.StatusActive {
		t.Errorf("Status = %q, want active", s.GetStatus())
	}
}
