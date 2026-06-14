package domain

import (
	"errors"
	"testing"
)

func validQuestion() Question {
	return Question{
		ID:            "Q1",
		Text:          "What is the past tense of 'go'?",
		Options:       []string{"goed", "went", "gone", "going"},
		CorrectAnswer: "went",
		Order:         1,
	}
}

func TestQuestion_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(q *Question)
		wantErr bool
	}{
		{"valid question", func(q *Question) {}, false},
		{"missing id", func(q *Question) { q.ID = "" }, true},
		{"missing text", func(q *Question) { q.Text = "" }, true},
		{"empty options", func(q *Question) { q.Options = nil }, true},
		{"single option", func(q *Question) { q.Options = []string{"went"} }, true},
		{"correct answer not in options", func(q *Question) { q.CorrectAnswer = "wendt" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := validQuestion()
			tt.mutate(&q)
			err := q.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestQuestion_IsCorrectAnswer(t *testing.T) {
	q := validQuestion()
	if !q.IsCorrectAnswer("went") {
		t.Errorf("IsCorrectAnswer(%q) = false, want true", "went")
	}
	if q.IsCorrectAnswer("goed") {
		t.Errorf("IsCorrectAnswer(%q) = true, want false", "goed")
	}
}

func TestQuestion_HasOption(t *testing.T) {
	q := validQuestion()
	if !q.HasOption("gone") {
		t.Errorf("HasOption(%q) = false, want true", "gone")
	}
	if q.HasOption("walked") {
		t.Errorf("HasOption(%q) = true, want false", "walked")
	}
}

func TestQuestion_Validate_WrapsErrInvalidQuestion(t *testing.T) {
	q := validQuestion()
	q.ID = ""
	if err := q.Validate(); !errors.Is(err, ErrInvalidQuestion) {
		t.Errorf("Validate() error = %v, want wrapping ErrInvalidQuestion", err)
	}
}
