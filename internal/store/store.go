package store

import "github.com/gray/elsa-quiz/internal/domain"

type Store interface {
	CreateSession(s *domain.QuizSession) error
	GetSession(id string) (*domain.QuizSession, error)
	ActiveSessionCount() int
}
