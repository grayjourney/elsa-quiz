package store

import (
	"sync"

	"github.com/gray/elsa-quiz/internal/domain"
)

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*domain.QuizSession
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]*domain.QuizSession)}
}

func (m *MemoryStore) CreateSession(s *domain.QuizSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[s.ID]; exists {
		return domain.ErrSessionExists
	}
	m.sessions[s.ID] = s
	return nil
}

func (m *MemoryStore) GetSession(id string) (*domain.QuizSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	return s, nil
}

func (m *MemoryStore) ActiveSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var n int
	for _, s := range m.sessions {
		if s.GetStatus() == domain.StatusActive {
			n++
		}
	}
	return n
}
