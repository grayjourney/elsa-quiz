package domain

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestQuizSession_ConcurrentSubmissions_NoRaceNoLostScores exercises the
// FR-1.3 / reliability requirement: many participants answering at once must
// not race or lose scores. Run with -race.
func TestQuizSession_ConcurrentSubmissions_NoRaceNoLostScores(t *testing.T) {
	const n = 100
	qs := []Question{{ID: "Q1", Text: "t", Options: []string{"wrong", "right"}, CorrectAnswer: "right", Order: 1}}
	s, err := NewQuizSession("QUIZ-LOAD", qs, EndPolicyManual, 0)
	if err != nil {
		t.Fatalf("NewQuizSession() = %v", err)
	}
	for i := range n {
		_, _ = s.AddParticipant(NewParticipant(fmt.Sprintf("u%d", i), s.ID, fmt.Sprintf("user%d", i)))
	}
	_ = s.Start(time.Now())

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = s.SubmitAnswer(fmt.Sprintf("u%d", i), "Q1", "right", 10, time.Now())
		}(i)
	}
	wg.Wait()

	var total int
	for _, p := range s.Participants() {
		total += p.Score
	}
	if total != n*10 {
		t.Errorf("total score = %d, want %d (no scores lost)", total, n*10)
	}
}
