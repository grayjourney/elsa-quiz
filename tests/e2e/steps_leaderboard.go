//go:build e2e

package e2e

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func registerLeaderboardSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^the participants have answered correctly:$`, w.participantsAnsweredCorrectly)
	ctx.Step(`^the leaderboard is calculated$`, func() error { return nil })
	ctx.Step(`^the leaderboard ranks the participants:$`, w.leaderboardRanks)
	ctx.Step(`^"([^"]*)" answers (\d+) questions correctly$`, w.answersNCorrectly)
	ctx.Step(`^any connected participant's score changes$`, w.anyConnectedScores)
	ctx.Step(`^all (\d+) participants receive the updated leaderboard$`, w.allReceiveLeaderboard)
	ctx.Step(`^"([^"]*)" reaches score (\d+) before "([^"]*)"$`, w.reachesScoreBefore)
	ctx.Step(`^"([^"]*)" is ranked above "([^"]*)"$`, w.rankedAbove)
	ctx.Step(`^"([^"]*)", "([^"]*)", and "([^"]*)" each reach score (\d+) in that order$`, w.threeReachScore)
}

// participantsAnsweredCorrectly reads "Name: N" lines and gives each player N
// correct answers, advancing the (manual) quiz between questions.
func (w *World) participantsAnsweredCorrectly(doc *godog.DocString) error {
	type target struct {
		name string
		n    int
	}
	var targets []target
	maxN := 0
	for _, line := range strings.Split(strings.TrimSpace(doc.Content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("bad participants line %q", line)
		}
		name := strings.TrimSpace(parts[0])
		n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return fmt.Errorf("bad count in %q: %w", line, err)
		}
		if err := w.joinREST(name); err != nil {
			return err
		}
		targets = append(targets, target{name, n})
		if n > maxN {
			maxN = n
		}
	}
	for i := 1; i <= maxN; i++ {
		qid, err := w.currentQID()
		if err != nil {
			return err
		}
		for _, t := range targets {
			if t.n >= i {
				if err := w.submitREST(t.name, qid, correctAnswerFor(qid)); err != nil {
					return err
				}
				if w.lastStatus != 200 {
					return fmt.Errorf("%s answer %d status = %d (%s)", t.name, i, w.lastStatus, w.lastErr.Code)
				}
			}
		}
		if i < maxN {
			if err := w.advanceREST(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *World) leaderboardRanks(doc *godog.DocString) error {
	var want []string
	for _, line := range strings.Split(strings.TrimSpace(doc.Content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// "1. Bob" -> "bob"
		if i := strings.Index(line, "."); i >= 0 {
			line = line[i+1:]
		}
		want = append(want, userIDFor(strings.TrimSpace(line)))
	}
	board, err := w.leaderboard()
	if err != nil {
		return err
	}
	if len(board) < len(want) {
		return fmt.Errorf("leaderboard has %d entries, want at least %d", len(board), len(want))
	}
	for i, uid := range want {
		if board[i].UserID != uid {
			return fmt.Errorf("rank %d = %q, want %q (board: %+v)", i+1, board[i].UserID, uid, board)
		}
	}
	return nil
}

func (w *World) answersNCorrectly(name string, n int) error {
	for j := 1; j <= n; j++ {
		qid, err := w.currentQID()
		if err != nil {
			return err
		}
		if err := w.submitREST(name, qid, correctAnswerFor(qid)); err != nil {
			return err
		}
		if w.lastStatus != 200 {
			return fmt.Errorf("%s answer %d status = %d (%s)", name, j, w.lastStatus, w.lastErr.Code)
		}
		if j < n {
			if err := w.advanceREST(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *World) anyConnectedScores() error {
	if len(w.connected) == 0 {
		return fmt.Errorf("no connected participants")
	}
	qid, err := w.currentQID()
	if err != nil {
		return err
	}
	_, err = w.wsSubmitAwaitScore(w.connected[0], qid, correctAnswerFor(qid))
	return err
}

func (w *World) allReceiveLeaderboard(_ int) error {
	for _, name := range w.connected {
		p := w.participantFor(name)
		if p.ws == nil {
			return fmt.Errorf("%s has no websocket", name)
		}
		if _, ok := p.ws.waitFor(msgLeaderboardUpdate, wsTimeout); !ok {
			return fmt.Errorf("%s did not receive a leaderboard_update", name)
		}
	}
	return nil
}

func (w *World) reachesScoreBefore(first string, _ int, second string) error {
	for _, name := range []string{first, second} {
		if err := w.joinREST(name); err != nil {
			return err
		}
	}
	qid, err := w.currentQID()
	if err != nil {
		return err
	}
	if err := w.submitREST(first, qid, correctAnswerFor(qid)); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("%s answer status = %d (%s)", first, w.lastStatus, w.lastErr.Code)
	}
	if err := w.submitREST(second, qid, correctAnswerFor(qid)); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("%s answer status = %d (%s)", second, w.lastStatus, w.lastErr.Code)
	}
	return nil
}

func (w *World) rankedAbove(higher, lower string) error {
	board, err := w.leaderboard()
	if err != nil {
		return err
	}
	hi, lo := -1, -1
	for i, e := range board {
		switch e.UserID {
		case userIDFor(higher):
			hi = i
		case userIDFor(lower):
			lo = i
		}
	}
	if hi < 0 || lo < 0 {
		return fmt.Errorf("missing participants on board: %+v", board)
	}
	if hi >= lo {
		return fmt.Errorf("%s (rank %d) not ranked above %s (rank %d)", higher, hi+1, lower, lo+1)
	}
	return nil
}

func (w *World) threeReachScore(a, b, c string, _ int) error {
	for _, name := range []string{a, b, c} {
		if err := w.joinREST(name); err != nil {
			return err
		}
	}
	qid, err := w.currentQID()
	if err != nil {
		return err
	}
	for _, name := range []string{a, b, c} {
		if err := w.submitREST(name, qid, correctAnswerFor(qid)); err != nil {
			return err
		}
		if w.lastStatus != 200 {
			return fmt.Errorf("%s answer status = %d (%s)", name, w.lastStatus, w.lastErr.Code)
		}
	}
	return nil
}
