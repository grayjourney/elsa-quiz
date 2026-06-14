package domain

import (
	"errors"
	"testing"
	"time"
)

const testBasePoints = 10

func twoQuestionSession(t *testing.T, policy EndPolicy, limit time.Duration) *QuizSession {
	t.Helper()
	qs := []Question{
		{ID: "Q1", Text: "past tense of go", Options: []string{"goed", "went"}, CorrectAnswer: "went", Order: 1},
		{ID: "Q2", Text: "plural of mouse", Options: []string{"mouses", "mice"}, CorrectAnswer: "mice", Order: 2},
	}
	s, err := NewQuizSession("QUIZ-ABC", qs, policy, limit)
	if err != nil {
		t.Fatalf("NewQuizSession() = %v, want nil", err)
	}
	return s
}

func TestNewQuizSession_StatusIsWaiting(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	if s.Status != StatusWaiting {
		t.Errorf("Status = %q, want %q", s.Status, StatusWaiting)
	}
}

func TestNewQuizSession_TimedRequiresPositiveLimit(t *testing.T) {
	qs := []Question{{ID: "Q1", Text: "t", Options: []string{"a", "b"}, CorrectAnswer: "a"}}
	if _, err := NewQuizSession("QUIZ-ABC", qs, EndPolicyTimed, 0); !errors.Is(err, ErrInvalidTimeLimit) {
		t.Errorf("NewQuizSession(timed, 0) = %v, want ErrInvalidTimeLimit", err)
	}
	if _, err := NewQuizSession("QUIZ-ABC", qs, EndPolicyTimed, 30*time.Second); err != nil {
		t.Errorf("NewQuizSession(timed, 30s) = %v, want nil", err)
	}
}

func TestQuizSession_AddParticipant_AddsToList(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	if _, err := s.AddParticipant(NewParticipant("u1", s.ID, "Alice")); err != nil {
		t.Fatalf("AddParticipant() = %v, want nil", err)
	}
	if got := len(s.Participants()); got != 1 {
		t.Errorf("participant count = %d, want 1", got)
	}
}

func TestQuizSession_AddParticipant_ToCompletedSession_ReturnsError(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	s.Complete()
	_, err := s.AddParticipant(NewParticipant("u1", s.ID, "Alice"))
	if !errors.Is(err, ErrSessionEnded) {
		t.Errorf("AddParticipant() to completed = %v, want ErrSessionEnded", err)
	}
}

func TestQuizSession_AddParticipant_Idempotent_PreservesExisting(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	p1, _ := s.AddParticipant(NewParticipant("u1", s.ID, "Alice"))
	p1.AddScore(30, time.Now())

	// Re-join with a fresh participant object (a reconnect) must NOT reset state.
	got, err := s.AddParticipant(NewParticipant("u1", s.ID, "Alice"))
	if err != nil {
		t.Fatalf("rejoin = %v, want nil", err)
	}
	if got.Score != 30 {
		t.Errorf("rejoined score = %d, want 30 (existing preserved)", got.Score)
	}
	if n := len(s.Participants()); n != 1 {
		t.Errorf("participant count = %d, want 1 (not duplicated)", n)
	}
}

func TestQuizSession_Start_ChangesStatusToActiveAndExposesFirstQuestion(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	if err := s.Start(time.Now()); err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	if s.Status != StatusActive {
		t.Errorf("Status = %q, want %q", s.Status, StatusActive)
	}
	q, ok := s.CurrentQuestion()
	if !ok || q.ID != "Q1" {
		t.Errorf("CurrentQuestion() = (%v, %v), want (Q1, true)", q.ID, ok)
	}
}

func TestQuizSession_Start_WhenNotWaiting_ReturnsError(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	_ = s.Start(time.Now())
	if err := s.Start(time.Now()); !errors.Is(err, ErrInvalidSessionState) {
		t.Errorf("second Start() = %v, want ErrInvalidSessionState", err)
	}
}

func TestQuizSession_AdvanceQuestion_MovesToNextThenCompletes(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	_ = s.Start(time.Now())

	q, ongoing, err := s.AdvanceQuestion(time.Now())
	if err != nil || !ongoing || q.ID != "Q2" {
		t.Fatalf("AdvanceQuestion() = (%v, %v, %v), want (Q2, true, nil)", q.ID, ongoing, err)
	}

	_, ongoing, err = s.AdvanceQuestion(time.Now())
	if err != nil || ongoing {
		t.Fatalf("AdvanceQuestion() past last = (ongoing %v, err %v), want (false, nil)", ongoing, err)
	}
	if s.Status != StatusCompleted {
		t.Errorf("Status after advancing past last = %q, want %q", s.Status, StatusCompleted)
	}
}

func TestQuizSession_SubmitAnswer_Correct_IncreasesScoreAndStampsTime(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	_, _ = s.AddParticipant(NewParticipant("u1", s.ID, "Alice"))
	_ = s.Start(time.Now())
	at := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)

	res, err := s.SubmitAnswer("u1", "Q1", "went", testBasePoints, at)
	if err != nil {
		t.Fatalf("SubmitAnswer() = %v, want nil", err)
	}
	if !res.IsCorrect || res.AwardedPoints != 10 || res.NewScore != 10 {
		t.Errorf("result = %+v, want {correct, +10, total 10}", res)
	}
	p, _ := s.Participant("u1")
	if !p.LastScoredAt.Equal(at) {
		t.Errorf("LastScoredAt = %v, want %v", p.LastScoredAt, at)
	}
}

func TestQuizSession_SubmitAnswer_Incorrect_NoScoreChange(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyManual, 0)
	_, _ = s.AddParticipant(NewParticipant("u1", s.ID, "Alice"))
	_ = s.Start(time.Now())

	res, err := s.SubmitAnswer("u1", "Q1", "goed", testBasePoints, time.Now())
	if err != nil {
		t.Fatalf("SubmitAnswer() = %v, want nil", err)
	}
	if res.IsCorrect || res.NewScore != 0 {
		t.Errorf("result = %+v, want {incorrect, total 0}", res)
	}
}

func TestQuizSession_SubmitAnswer_Errors(t *testing.T) {
	setup := func(t *testing.T) *QuizSession {
		s := twoQuestionSession(t, EndPolicyManual, 0)
		_, _ = s.AddParticipant(NewParticipant("u1", s.ID, "Alice"))
		_ = s.Start(time.Now())
		return s
	}

	t.Run("duplicate answer", func(t *testing.T) {
		s := setup(t)
		_, _ = s.SubmitAnswer("u1", "Q1", "went", testBasePoints, time.Now())
		_, err := s.SubmitAnswer("u1", "Q1", "goed", testBasePoints, time.Now())
		if !errors.Is(err, ErrDuplicateAnswer) {
			t.Errorf("err = %v, want ErrDuplicateAnswer", err)
		}
	})

	t.Run("empty answer", func(t *testing.T) {
		s := setup(t)
		_, err := s.SubmitAnswer("u1", "Q1", "", testBasePoints, time.Now())
		if !errors.Is(err, ErrAnswerEmpty) {
			t.Errorf("err = %v, want ErrAnswerEmpty", err)
		}
	})

	t.Run("invalid option", func(t *testing.T) {
		s := setup(t)
		_, err := s.SubmitAnswer("u1", "Q1", "walked", testBasePoints, time.Now())
		if !errors.Is(err, ErrInvalidOption) {
			t.Errorf("err = %v, want ErrInvalidOption", err)
		}
	})

	t.Run("non-current/unknown question", func(t *testing.T) {
		s := setup(t)
		_, err := s.SubmitAnswer("u1", "Q-NONEXISTENT", "went", testBasePoints, time.Now())
		if !errors.Is(err, ErrQuestionNotFound) {
			t.Errorf("err = %v, want ErrQuestionNotFound", err)
		}
	})

	t.Run("unknown participant", func(t *testing.T) {
		s := setup(t)
		_, err := s.SubmitAnswer("ghost", "Q1", "went", testBasePoints, time.Now())
		if !errors.Is(err, ErrParticipantNotFound) {
			t.Errorf("err = %v, want ErrParticipantNotFound (not session-not-found — the session exists)", err)
		}
	})

	t.Run("after completion", func(t *testing.T) {
		s := setup(t)
		s.Complete()
		_, err := s.SubmitAnswer("u1", "Q1", "went", testBasePoints, time.Now())
		if !errors.Is(err, ErrQuizEnded) {
			t.Errorf("err = %v, want ErrQuizEnded", err)
		}
	})
}

func TestQuizSession_SubmitAnswer_TimedAfterLimit_ReturnsTimeUp(t *testing.T) {
	s := twoQuestionSession(t, EndPolicyTimed, 30*time.Second)
	_, _ = s.AddParticipant(NewParticipant("u1", s.ID, "Alice"))
	opened := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	_ = s.Start(opened)

	late := opened.Add(31 * time.Second)
	_, err := s.SubmitAnswer("u1", "Q1", "went", testBasePoints, late)
	if !errors.Is(err, ErrTimeUp) {
		t.Errorf("late submit err = %v, want ErrTimeUp", err)
	}

	s2 := twoQuestionSession(t, EndPolicyTimed, 30*time.Second)
	_, _ = s2.AddParticipant(NewParticipant("u1", s2.ID, "Alice"))
	_ = s2.Start(opened)
	if _, err := s2.SubmitAnswer("u1", "Q1", "went", testBasePoints, opened.Add(5*time.Second)); err != nil {
		t.Errorf("in-time submit err = %v, want nil", err)
	}
}
