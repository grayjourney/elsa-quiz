package service

import (
	"fmt"
	"time"

	"github.com/gray/elsa-quiz/internal/domain"
	"github.com/gray/elsa-quiz/internal/store"
	"github.com/gray/elsa-quiz/pkg/id"
)

type SessionService struct {
	store store.Store
}

func NewSessionService(st store.Store) *SessionService {
	return &SessionService{store: st}
}

func (svc *SessionService) CreateSession(questions []domain.Question, policy domain.EndPolicy, timeLimit time.Duration) (*domain.QuizSession, error) {
	s, err := domain.NewQuizSession("QUIZ-"+id.New(), questions, policy, timeLimit)
	if err != nil {
		return nil, err
	}
	if err := svc.store.CreateSession(s); err != nil {
		return nil, fmt.Errorf("persisting session: %w", err)
	}
	return s, nil
}

func (svc *SessionService) JoinSession(quizID, userID, displayName string) (*domain.Participant, error) {
	if quizID == "" {
		return nil, domain.ErrQuizIDRequired
	}
	s, err := svc.store.GetSession(quizID)
	if err != nil {
		return nil, err
	}
	p, err := s.AddParticipant(domain.NewParticipant(userID, quizID, displayName))
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (svc *SessionService) StartSession(quizID string, at time.Time) error {
	s, err := svc.store.GetSession(quizID)
	if err != nil {
		return err
	}
	return s.Start(at)
}
