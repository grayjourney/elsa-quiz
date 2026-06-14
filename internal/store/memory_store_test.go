package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gray/elsa-quiz/internal/domain"
)

func newSession(t *testing.T, id string) *domain.QuizSession {
	t.Helper()
	qs := []domain.Question{{ID: "Q1", Text: "t", Options: []string{"a", "b"}, CorrectAnswer: "a"}}
	s, err := domain.NewQuizSession(id, qs, domain.EndPolicyManual, 0)
	if err != nil {
		t.Fatalf("NewQuizSession() = %v", err)
	}
	return s
}

func TestMemoryStore_CreateAndGetSession(t *testing.T) {
	st := NewMemoryStore()
	s := newSession(t, "QUIZ-ABC")
	if err := st.CreateSession(s); err != nil {
		t.Fatalf("CreateSession() = %v, want nil", err)
	}
	got, err := st.GetSession("QUIZ-ABC")
	if err != nil {
		t.Fatalf("GetSession() = %v, want nil", err)
	}
	if got.ID != "QUIZ-ABC" {
		t.Errorf("GetSession().ID = %q, want QUIZ-ABC", got.ID)
	}
}

func TestMemoryStore_GetSession_NotFound(t *testing.T) {
	st := NewMemoryStore()
	if _, err := st.GetSession("QUIZ-999"); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("GetSession(missing) = %v, want ErrSessionNotFound", err)
	}
}

func TestMemoryStore_CreateSession_Duplicate(t *testing.T) {
	st := NewMemoryStore()
	s := newSession(t, "QUIZ-ABC")
	_ = st.CreateSession(s)
	if err := st.CreateSession(newSession(t, "QUIZ-ABC")); !errors.Is(err, domain.ErrSessionExists) {
		t.Errorf("duplicate CreateSession() = %v, want ErrSessionExists", err)
	}
}

func TestMemoryStore_ActiveSessionCount(t *testing.T) {
	st := NewMemoryStore()
	waiting := newSession(t, "QUIZ-1")
	active := newSession(t, "QUIZ-2")
	_ = active.Start(time.Now())
	_ = st.CreateSession(waiting)
	_ = st.CreateSession(active)
	if got := st.ActiveSessionCount(); got != 1 {
		t.Errorf("ActiveSessionCount() = %d, want 1", got)
	}
}

func TestMemoryStore_ConcurrentAccess_NoRace(t *testing.T) {
	st := NewMemoryStore()
	const n = 100
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("QUIZ-%d", i)
			_ = st.CreateSession(newSession(t, id))
			_, _ = st.GetSession(id)
			_ = st.ActiveSessionCount()
		}(i)
	}
	wg.Wait()
}
