package handler

import (
	"sync"
	"testing"
	"time"
)

func TestScheduler_FiresAfterDuration(t *testing.T) {
	fired := make(chan string, 1)
	s := newScheduler(func(id string) { fired <- id })
	s.schedule("QUIZ-1", 30*time.Millisecond)
	select {
	case id := <-fired:
		if id != "QUIZ-1" {
			t.Errorf("fired with %q, want QUIZ-1", id)
		}
	case <-time.After(time.Second):
		t.Fatal("timer never fired")
	}
}

func TestScheduler_CancelPreventsFire(t *testing.T) {
	fired := make(chan string, 1)
	s := newScheduler(func(id string) { fired <- id })
	s.schedule("QUIZ-1", 40*time.Millisecond)
	s.cancel("QUIZ-1")
	select {
	case <-fired:
		t.Error("fired after cancel")
	case <-time.After(120 * time.Millisecond):
	}
}

func TestScheduler_RescheduleReplacesPrevious(t *testing.T) {
	var mu sync.Mutex
	count := 0
	s := newScheduler(func(string) { mu.Lock(); count++; mu.Unlock() })
	s.schedule("QUIZ-1", 20*time.Millisecond)
	s.schedule("QUIZ-1", 70*time.Millisecond) // replaces the 20ms timer
	time.Sleep(140 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Errorf("fired %d times, want exactly 1", count)
	}
}

func TestConnectionManager_UserIDs(t *testing.T) {
	m := NewConnectionManager()
	m.Register("QUIZ-1", "alice", newFakeConn())
	m.Register("QUIZ-1", "bob", newFakeConn())
	ids := m.UserIDs("QUIZ-1")
	if len(ids) != 2 {
		t.Fatalf("UserIDs = %v, want 2", ids)
	}
}
