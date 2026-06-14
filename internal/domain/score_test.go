package domain

import "testing"

func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name       string
		isCorrect  bool
		basePoints int
		want       int
	}{
		{"correct answer returns base points", true, 10, 10},
		{"incorrect answer returns zero", false, 10, 0},
		{"correct with different base", true, 25, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateScore(tt.isCorrect, tt.basePoints); got != tt.want {
				t.Errorf("CalculateScore(%v, %d) = %d, want %d", tt.isCorrect, tt.basePoints, got, tt.want)
			}
		})
	}
}
