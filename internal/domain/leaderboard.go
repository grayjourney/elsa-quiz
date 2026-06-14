package domain

import (
	"cmp"
	"slices"
	"time"
)

type LeaderboardEntry struct {
	Rank        int
	UserID      string
	DisplayName string
	Score       int
}

func CalculateLeaderboard(participants []Participant) []LeaderboardEntry {
	entries := make([]LeaderboardEntry, 0, len(participants))
	sorted := slices.Clone(participants)
	slices.SortFunc(sorted, func(a, b Participant) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}
		if c := compareTime(a.LastScoredAt, b.LastScoredAt); c != 0 {
			return c
		}
		return cmp.Compare(a.UserID, b.UserID)
	})
	for i, p := range sorted {
		entries = append(entries, LeaderboardEntry{
			Rank:        i + 1,
			UserID:      p.UserID,
			DisplayName: p.DisplayName,
			Score:       p.Score,
		})
	}
	return entries
}

func compareTime(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	default:
		return 0
	}
}
