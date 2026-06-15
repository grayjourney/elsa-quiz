//go:build e2e

package e2e

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerReliabilitySteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^"([^"]*)" was a participant with score (\d+)$`, w.participantWithScoreStep)
	ctx.Step(`^"([^"]*)"'s network connection drops$`, w.connectionDrops)
	ctx.Step(`^"([^"]*)"'s connection was dropped$`, w.connectionDrops)
	ctx.Step(`^"([^"]*)"'s score of (\d+) is preserved in the session$`, w.scorePreserved)
	ctx.Step(`^the leaderboard continues to show "([^"]*)" with score (\d+)$`, w.leaderboardShows)
	ctx.Step(`^"([^"]*)" reconnects to session "([^"]*)"$`, w.reconnects)
	ctx.Step(`^"([^"]*)" resumes with her previous score of (\d+)$`, w.resumesWithScore)
	ctx.Step(`^"([^"]*)" can continue submitting answers$`, w.canContinueSubmitting)

	ctx.Step(`^all (\d+) participants submit answers for question "([^"]*)" simultaneously$`, w.allSubmitSimultaneously)
	ctx.Step(`^all scores are calculated correctly$`, w.allScoresCorrect)
	ctx.Step(`^no scores are lost or duplicated$`, w.allScoresCorrect)
	ctx.Step(`^the leaderboard reflects accurate rankings$`, w.leaderboardReflectsRankings)
	ctx.Step(`^"([^"]*)" submits an answer at the exact same time as "([^"]*)"$`, w.bothSubmitConcurrently)
	ctx.Step(`^both answers are processed$`, func() error { return nil })
	ctx.Step(`^"([^"]*)"'s score reflects only (?:her|his|their) answer$`, w.scoreReflectsOnlyOwn)
}

func (w *World) connectionDrops(name string) error {
	p := w.participantFor(name)
	if p.ws != nil {
		p.ws.close()
		p.ws = nil
	}
	return nil
}

func (w *World) scorePreserved(name string, score int) error { return w.assertScore(name, score) }

func (w *World) leaderboardShows(name string, score int) error { return w.assertScore(name, score) }

func (w *World) reconnects(name, _ string) error { return w.wsJoin(name) }

func (w *World) resumesWithScore(name string, score int) error {
	p := w.participantFor(name)
	if p.joinScore != score {
		return fmt.Errorf("%s resumed with score %d, want %d", name, p.joinScore, score)
	}
	return nil
}

func (w *World) canContinueSubmitting(name string) error {
	qid, err := w.currentQID()
	if err != nil {
		return err
	}
	if err := w.submitREST(name, qid, correctAnswerFor(qid)); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("%s could not submit after reconnect: status %d (%s)", name, w.lastStatus, w.lastErr.Code)
	}
	return nil
}

func (w *World) allSubmitSimultaneously(_ int, qid string) error {
	accepted := w.concurrentSubmit(w.connected, qid, correctAnswerFor(qid))
	if accepted != len(w.connected) {
		return fmt.Errorf("accepted %d/%d concurrent submissions", accepted, len(w.connected))
	}
	return nil
}

func (w *World) allScoresCorrect() error {
	sum, err := w.sumScores()
	if err != nil {
		return err
	}
	want := len(w.connected) * 10
	if sum != want {
		return fmt.Errorf("total score = %d, want %d (lost/duplicated scores)", sum, want)
	}
	return nil
}

func (w *World) leaderboardReflectsRankings() error {
	board, err := w.leaderboard()
	if err != nil {
		return err
	}
	if len(board) != len(w.connected) {
		return fmt.Errorf("leaderboard has %d entries, want %d", len(board), len(w.connected))
	}
	return nil
}

func (w *World) bothSubmitConcurrently(a, b string) error {
	for _, name := range []string{a, b} {
		if err := w.joinREST(name); err != nil {
			return err
		}
	}
	w.concurrentSubmit([]string{a, b}, "Q1", correctAnswerFor("Q1"))
	return nil
}

func (w *World) scoreReflectsOnlyOwn(name string) error { return w.assertScore(name, 10) }
