package domain

import (
	"fmt"
	"slices"
	"sync"
	"time"
)

type Question struct {
	ID            string
	Text          string
	Options       []string
	CorrectAnswer string
	Order         int
}

func (q Question) Validate() error {
	switch {
	case q.ID == "":
		return fmt.Errorf("%w: missing id", ErrInvalidQuestion)
	case q.Text == "":
		return fmt.Errorf("%w: missing text", ErrInvalidQuestion)
	case len(q.Options) < 2:
		return fmt.Errorf("%w: need at least two options", ErrInvalidQuestion)
	case !q.HasOption(q.CorrectAnswer):
		return fmt.Errorf("%w: correct answer not among options", ErrInvalidQuestion)
	}
	return nil
}

func (q Question) IsCorrectAnswer(answer string) bool {
	return answer == q.CorrectAnswer
}

func (q Question) HasOption(answer string) bool {
	return slices.Contains(q.Options, answer)
}

type SessionStatus string

const (
	StatusWaiting   SessionStatus = "waiting"
	StatusActive    SessionStatus = "active"
	StatusCompleted SessionStatus = "completed"
)

type EndPolicy string

const (
	EndPolicyManual EndPolicy = "manual"
	EndPolicyTimed  EndPolicy = "timed"
)

type AnswerResult struct {
	IsCorrect     bool
	AwardedPoints int
	NewScore      int
}

type QuizSession struct {
	mu        sync.Mutex
	ID        string
	Status    SessionStatus
	EndPolicy EndPolicy
	TimeLimit time.Duration
	Questions []Question
	CreatedAt time.Time

	currentIdx   int
	openedAt     time.Time
	participants map[string]*Participant
}

func NewQuizSession(id string, questions []Question, policy EndPolicy, timeLimit time.Duration) (*QuizSession, error) {
	if policy == EndPolicyTimed && timeLimit <= 0 {
		return nil, ErrInvalidTimeLimit
	}
	return &QuizSession{
		ID:           id,
		Status:       StatusWaiting,
		EndPolicy:    policy,
		TimeLimit:    timeLimit,
		Questions:    questions,
		CreatedAt:    time.Now(),
		currentIdx:   -1,
		participants: make(map[string]*Participant),
	}, nil
}

// AddParticipant registers p and returns it. If a participant with the same
// UserID already joined, the existing one is returned unchanged (a reconnect):
// their score and answered-set are preserved across a dropped connection.
func (s *QuizSession) AddParticipant(p *Participant) (*Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Status == StatusCompleted {
		return nil, ErrSessionEnded
	}
	if existing, ok := s.participants[p.UserID]; ok {
		return existing, nil
	}
	s.participants[p.UserID] = p
	return p, nil
}

func (s *QuizSession) Participant(userID string) (*Participant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.participants[userID]
	return p, ok
}

func (s *QuizSession) Participants() []Participant {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Participant, 0, len(s.participants))
	for _, p := range s.participants {
		out = append(out, *p)
	}
	return out
}

func (s *QuizSession) GetStatus() SessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status
}

func (s *QuizSession) Start(at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Status != StatusWaiting {
		return ErrInvalidSessionState
	}
	s.Status = StatusActive
	s.currentIdx = 0
	s.openedAt = at
	return nil
}

// indexOfQuestionLocked returns the position of the question with id in the
// quiz, or -1 if there is no such question. Caller must hold s.mu.
func (s *QuizSession) indexOfQuestionLocked(id string) int {
	for i := range s.Questions {
		if s.Questions[i].ID == id {
			return i
		}
	}
	return -1
}

func (s *QuizSession) currentQuestionLocked() (Question, bool) {
	if s.Status != StatusActive || s.currentIdx < 0 || s.currentIdx >= len(s.Questions) {
		return Question{}, false
	}
	return s.Questions[s.currentIdx], true
}

func (s *QuizSession) CurrentQuestion() (Question, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentQuestionLocked()
}

// AdvanceQuestion moves to the next question. The bool reports whether the quiz
// is still ongoing; advancing past the final question completes the session.
func (s *QuizSession) AdvanceQuestion(at time.Time) (Question, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Status != StatusActive {
		return Question{}, false, ErrInvalidSessionState
	}
	if s.currentIdx+1 >= len(s.Questions) {
		s.Status = StatusCompleted
		return Question{}, false, nil
	}
	s.currentIdx++
	s.openedAt = at
	return s.Questions[s.currentIdx], true, nil
}

func (s *QuizSession) Complete() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = StatusCompleted
}

// AdvanceIfCurrent advances only if questionID is still the current question.
// advanced=false means another trigger (timer, all-answered, or host) already
// moved past it — making concurrent advance triggers safe. Advancing past the
// final question completes the session (ongoing=false, advanced=true).
func (s *QuizSession) AdvanceIfCurrent(questionID string, at time.Time) (next Question, ongoing, advanced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.currentQuestionLocked()
	if !ok || cur.ID != questionID {
		return Question{}, false, false
	}
	if s.currentIdx+1 >= len(s.Questions) {
		s.Status = StatusCompleted
		return Question{}, false, true
	}
	s.currentIdx++
	s.openedAt = at
	return s.Questions[s.currentIdx], true, true
}

// AllAnsweredCurrent reports whether every user in userIDs has answered the
// current question. Used to advance a timed question early once all connected
// participants have answered. An empty set is never "all answered".
func (s *QuizSession) AllAnsweredCurrent(userIDs []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.currentQuestionLocked()
	if !ok || len(userIDs) == 0 {
		return false
	}
	for _, uid := range userIDs {
		p, ok := s.participants[uid]
		if !ok || !p.HasAnswered(cur.ID) {
			return false
		}
	}
	return true
}

func (s *QuizSession) SubmitAnswer(userID, questionID, answer string, basePoints int, at time.Time) (AnswerResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var zero AnswerResult
	if s.Status == StatusCompleted {
		return zero, ErrQuizEnded
	}
	if s.Status != StatusActive {
		return zero, ErrInvalidSessionState
	}
	p, ok := s.participants[userID]
	if !ok {
		return zero, ErrParticipantNotFound
	}
	q, ok := s.currentQuestionLocked()
	if !ok || q.ID != questionID {
		// In a timed quiz, an answer for a question we have already advanced past
		// is late — its window closed — so report time_up, not question_not_found.
		// (A question we have not reached yet, or an unknown id, is not-found.)
		if s.EndPolicy == EndPolicyTimed {
			if idx := s.indexOfQuestionLocked(questionID); idx >= 0 && idx < s.currentIdx {
				return zero, ErrTimeUp
			}
		}
		return zero, ErrQuestionNotFound
	}
	if s.EndPolicy == EndPolicyTimed && at.Sub(s.openedAt) > s.TimeLimit {
		return zero, ErrTimeUp
	}
	if answer == "" {
		return zero, ErrAnswerEmpty
	}
	if !q.HasOption(answer) {
		return zero, ErrInvalidOption
	}
	if p.HasAnswered(questionID) {
		return zero, ErrDuplicateAnswer
	}
	_ = p.RecordAnswer(questionID)

	isCorrect := q.IsCorrectAnswer(answer)
	points := CalculateScore(isCorrect, basePoints)
	if points > 0 {
		p.AddScore(points, at)
	}
	return AnswerResult{IsCorrect: isCorrect, AwardedPoints: points, NewScore: p.Score}, nil
}
