package domain

import (
	"testing"
	"time"
)

func participantWithScore(userID, name string, score int, scoredAt time.Time) Participant {
	return Participant{UserID: userID, DisplayName: name, Score: score, LastScoredAt: scoredAt}
}

func TestCalculateLeaderboard_RanksByScoreDescending(t *testing.T) {
	now := time.Now()
	ps := []Participant{
		participantWithScore("u1", "Alice", 30, now),
		participantWithScore("u2", "Bob", 50, now),
		participantWithScore("u3", "Charlie", 40, now),
	}
	got := CalculateLeaderboard(ps)
	want := []struct {
		rank int
		name string
		pts  int
	}{
		{1, "Bob", 50},
		{2, "Charlie", 40},
		{3, "Alice", 30},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Rank != w.rank || got[i].DisplayName != w.name || got[i].Score != w.pts {
			t.Errorf("entry %d = {rank:%d name:%q score:%d}, want {rank:%d name:%q score:%d}",
				i, got[i].Rank, got[i].DisplayName, got[i].Score, w.rank, w.name, w.pts)
		}
	}
}

func TestCalculateLeaderboard_TieBreaking_EarlierScoreWins(t *testing.T) {
	t1 := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Second)
	ps := []Participant{
		participantWithScore("u2", "Bob", 40, t2),
		participantWithScore("u1", "Alice", 40, t1),
	}
	got := CalculateLeaderboard(ps)
	if got[0].DisplayName != "Alice" || got[1].DisplayName != "Bob" {
		t.Errorf("tie order = [%q, %q], want [Alice, Bob] (earlier LastScoredAt first)",
			got[0].DisplayName, got[1].DisplayName)
	}
	if got[0].Rank != 1 || got[1].Rank != 2 {
		t.Errorf("ranks = [%d, %d], want [1, 2]", got[0].Rank, got[1].Rank)
	}
}

func TestCalculateLeaderboard_EmptyParticipants_ReturnsEmptySlice(t *testing.T) {
	got := CalculateLeaderboard(nil)
	if got == nil {
		t.Errorf("got nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestCalculateLeaderboard_SingleParticipant_RankOne(t *testing.T) {
	got := CalculateLeaderboard([]Participant{participantWithScore("u1", "Alice", 10, time.Now())})
	if len(got) != 1 || got[0].Rank != 1 {
		t.Fatalf("got %+v, want single entry with rank 1", got)
	}
}

func TestCalculateLeaderboard_DeterministicForFullTie(t *testing.T) {
	at := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	ps := []Participant{
		participantWithScore("u3", "Charlie", 40, at),
		participantWithScore("u1", "Alice", 40, at),
		participantWithScore("u2", "Bob", 40, at),
	}
	first := CalculateLeaderboard(ps)
	second := CalculateLeaderboard(ps)
	for i := range first {
		if first[i].UserID != second[i].UserID {
			t.Fatalf("non-deterministic ordering at %d: %q vs %q", i, first[i].UserID, second[i].UserID)
		}
	}
	if first[0].UserID != "u1" {
		t.Errorf("full-tie order = %q first, want u1 (UserID asc tiebreaker)", first[0].UserID)
	}
}
