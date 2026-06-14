package domain

func CalculateScore(isCorrect bool, basePoints int) int {
	if isCorrect {
		return basePoints
	}
	return 0
}
