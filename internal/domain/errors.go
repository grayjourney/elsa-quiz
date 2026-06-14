package domain

// Error carries a stable machine-readable Code and a user-facing Message that
// is safe to display directly. Sentinels are matched via errors.Is by identity.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

var (
	ErrSessionNotFound  = &Error{Code: "session_not_found", Message: "Quiz session not found"}
	ErrSessionEnded     = &Error{Code: "session_ended", Message: "Quiz session has already ended"}
	ErrQuizIDRequired   = &Error{Code: "quiz_id_required", Message: "Quiz ID is required"}
	ErrQuestionNotFound = &Error{Code: "question_not_found", Message: "Question not found"}
	ErrDuplicateAnswer  = &Error{Code: "duplicate_answer", Message: "Answer already submitted for this question"}
	ErrAnswerEmpty      = &Error{Code: "answer_empty", Message: "Answer cannot be empty"}
	ErrInvalidOption    = &Error{Code: "invalid_option", Message: "Invalid answer option"}
	ErrQuizEnded        = &Error{Code: "quiz_ended", Message: "Quiz has already ended"}
	ErrTimeUp           = &Error{Code: "time_up", Message: "Time is up for this question"}
	ErrInvalidTimeLimit = &Error{Code: "invalid_time_limit", Message: "Time limit must be greater than zero"}
	ErrInvalidQuestion     = &Error{Code: "invalid_question", Message: "Question is invalid"}
	ErrInvalidSessionState = &Error{Code: "invalid_session_state", Message: "Operation not allowed in the current session state"}
	ErrSessionExists       = &Error{Code: "session_exists", Message: "Quiz session already exists"}
	ErrParticipantNotFound = &Error{Code: "participant_not_found", Message: "Participant has not joined this quiz session"}
)
