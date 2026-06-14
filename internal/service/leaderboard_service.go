package service

import (
	"github.com/gray/elsa-quiz/internal/domain"
	"github.com/gray/elsa-quiz/internal/store"
)

type LeaderboardService struct {
	store store.Store
}

func NewLeaderboardService(st store.Store) *LeaderboardService {
	return &LeaderboardService{store: st}
}

func (svc *LeaderboardService) GetLeaderboard(quizID string) ([]domain.LeaderboardEntry, error) {
	s, err := svc.store.GetSession(quizID)
	if err != nil {
		return nil, err
	}
	return domain.CalculateLeaderboard(s.Participants()), nil
}
