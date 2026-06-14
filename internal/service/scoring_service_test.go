package service

import (
	"errors"
	"testing"
	"time"

	"github.com/gray/elsa-quiz/internal/domain"
	"github.com/gray/elsa-quiz/internal/store"
)

const basePoints = 10

// activeSessionWith sets up a started session with the given participants and
// returns the services wired to a shared store.
func activeSessionWith(t *testing.T, users ...string) (*SessionService, *ScoringService, *LeaderboardService, *domain.QuizSession) {
	t.Helper()
	st := store.NewMemoryStore()
	sessions := NewSessionService(st)
	scoring := NewScoringService(st, basePoints)
	board := NewLeaderboardService(st)

	s, err := sessions.CreateSession(sampleQuestions(), domain.EndPolicyManual, 0)
	if err != nil {
		t.Fatalf("CreateSession() = %v", err)
	}
	for _, u := range users {
		if _, err := sessions.JoinSession(s.ID, u, u); err != nil {
			t.Fatalf("JoinSession(%q) = %v", u, err)
		}
	}
	if err := sessions.StartSession(s.ID, time.Now()); err != nil {
		t.Fatalf("StartSession() = %v", err)
	}
	return sessions, scoring, board, s
}

func TestScoringService_SubmitAnswer_CorrectIncreasesScoreAndReturnsLeaderboard(t *testing.T) {
	_, scoring, _, s := activeSessionWith(t, "Alice", "Bob")

	res, board, err := scoring.SubmitAnswer(s.ID, "Alice", "Q1", "went", time.Now())
	if err != nil {
		t.Fatalf("SubmitAnswer() = %v, want nil", err)
	}
	if !res.IsCorrect || res.NewScore != 10 {
		t.Errorf("result = %+v, want correct with score 10", res)
	}
	if len(board) != 2 || board[0].DisplayName != "Alice" || board[0].Score != 10 {
		t.Errorf("leaderboard = %+v, want Alice top with 10", board)
	}
}

func TestScoringService_SubmitAnswer_IncorrectNoChange(t *testing.T) {
	_, scoring, _, s := activeSessionWith(t, "Alice")
	res, _, err := scoring.SubmitAnswer(s.ID, "Alice", "Q1", "goed", time.Now())
	if err != nil {
		t.Fatalf("SubmitAnswer() = %v, want nil", err)
	}
	if res.IsCorrect || res.NewScore != 0 {
		t.Errorf("result = %+v, want incorrect with score 0", res)
	}
}

func TestScoringService_SubmitAnswer_Errors(t *testing.T) {
	t.Run("duplicate", func(t *testing.T) {
		_, scoring, _, s := activeSessionWith(t, "Alice")
		_, _, _ = scoring.SubmitAnswer(s.ID, "Alice", "Q1", "went", time.Now())
		_, _, err := scoring.SubmitAnswer(s.ID, "Alice", "Q1", "goed", time.Now())
		if !errors.Is(err, domain.ErrDuplicateAnswer) {
			t.Errorf("err = %v, want ErrDuplicateAnswer", err)
		}
	})
	t.Run("invalid question", func(t *testing.T) {
		_, scoring, _, s := activeSessionWith(t, "Alice")
		_, _, err := scoring.SubmitAnswer(s.ID, "Alice", "Q-NOPE", "went", time.Now())
		if !errors.Is(err, domain.ErrQuestionNotFound) {
			t.Errorf("err = %v, want ErrQuestionNotFound", err)
		}
	})
	t.Run("session not found", func(t *testing.T) {
		_, scoring, _, _ := activeSessionWith(t, "Alice")
		_, _, err := scoring.SubmitAnswer("QUIZ-NOPE", "Alice", "Q1", "went", time.Now())
		if !errors.Is(err, domain.ErrSessionNotFound) {
			t.Errorf("err = %v, want ErrSessionNotFound", err)
		}
	})
}

func TestLeaderboardService_GetLeaderboard_ReturnsRanked(t *testing.T) {
	_, scoring, board, s := activeSessionWith(t, "Alice", "Bob")
	_, _, _ = scoring.SubmitAnswer(s.ID, "Alice", "Q1", "went", time.Now())

	got, err := board.GetLeaderboard(s.ID)
	if err != nil {
		t.Fatalf("GetLeaderboard() = %v, want nil", err)
	}
	if len(got) != 2 || got[0].DisplayName != "Alice" || got[0].Rank != 1 {
		t.Errorf("leaderboard = %+v, want Alice rank 1", got)
	}
}

func TestLeaderboardService_GetLeaderboard_NotFound(t *testing.T) {
	board := NewLeaderboardService(store.NewMemoryStore())
	if _, err := board.GetLeaderboard("QUIZ-NOPE"); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
}
