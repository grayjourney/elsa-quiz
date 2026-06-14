package handler

import (
	"sync"
	"time"
)

// scheduler manages one pending per-session timer for timed quizzes. Scheduling
// a session replaces its previous timer; firing invokes onFire in a new goroutine.
type scheduler struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	onFire func(quizID string)
}

func newScheduler(onFire func(quizID string)) *scheduler {
	return &scheduler{timers: make(map[string]*time.Timer), onFire: onFire}
}

func (s *scheduler) schedule(quizID string, d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[quizID]; ok {
		t.Stop()
	}
	s.timers[quizID] = time.AfterFunc(d, func() { s.onFire(quizID) })
}

func (s *scheduler) cancel(quizID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[quizID]; ok {
		t.Stop()
		delete(s.timers, quizID)
	}
}

func (s *scheduler) cancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.timers {
		t.Stop()
		delete(s.timers, id)
	}
}
