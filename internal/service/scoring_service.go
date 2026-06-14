package service

import (
	"time"

	"github.com/gray/elsa-quiz/internal/domain"
	"github.com/gray/elsa-quiz/internal/store"
)

type ScoringService struct {
	store      store.Store
	basePoints int
}

func NewScoringService(st store.Store, basePoints int) *ScoringService {
	return &ScoringService{store: st, basePoints: basePoints}
}

func (svc *ScoringService) SubmitAnswer(quizID, userID, questionID, answer string, at time.Time) (domain.AnswerResult, []domain.LeaderboardEntry, error) {
	s, err := svc.store.GetSession(quizID)
	if err != nil {
		return domain.AnswerResult{}, nil, err
	}
	res, err := s.SubmitAnswer(userID, questionID, answer, svc.basePoints, at)
	if err != nil {
		return domain.AnswerResult{}, nil, err
	}
	return res, domain.CalculateLeaderboard(s.Participants()), nil
}
