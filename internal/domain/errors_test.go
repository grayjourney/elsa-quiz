package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestDomainError_Error_ReturnsUserFacingMessage(t *testing.T) {
	tests := []struct {
		err     *Error
		code    string
		message string
	}{
		{ErrSessionNotFound, "session_not_found", "Quiz session not found"},
		{ErrSessionEnded, "session_ended", "Quiz session has already ended"},
		{ErrQuizIDRequired, "quiz_id_required", "Quiz ID is required"},
		{ErrQuestionNotFound, "question_not_found", "Question not found"},
		{ErrDuplicateAnswer, "duplicate_answer", "Answer already submitted for this question"},
		{ErrAnswerEmpty, "answer_empty", "Answer cannot be empty"},
		{ErrInvalidOption, "invalid_option", "Invalid answer option"},
		{ErrQuizEnded, "quiz_ended", "Quiz has already ended"},
		{ErrTimeUp, "time_up", "Time is up for this question"},
		{ErrInvalidTimeLimit, "invalid_time_limit", "Time limit must be greater than zero"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.code)
			}
			if tt.err.Error() != tt.message {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.message)
			}
		})
	}
}

func TestDomainError_IsMatchesWhenWrapped(t *testing.T) {
	wrapped := fmt.Errorf("joining session: %w", ErrSessionNotFound)
	if !errors.Is(wrapped, ErrSessionNotFound) {
		t.Errorf("errors.Is(wrapped, ErrSessionNotFound) = false, want true")
	}
	if errors.Is(wrapped, ErrQuizEnded) {
		t.Errorf("errors.Is(wrapped, ErrQuizEnded) = true, want false")
	}
}
