//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"

	"github.com/cucumber/godog"
)

func registerScoreSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^a quiz session "([^"]*)" is active$`, w.activePlain)
	ctx.Step(`^"([^"]*)" is a participant with score (\d+)$`, w.participantWithScoreStep)
	ctx.Step(`^"([^"]*)" is connected$`, w.connectedStep)
	ctx.Step(`^"([^"]*)" and "([^"]*)" both have score 0$`, w.bothScoreZero)

	ctx.Step(`^"([^"]*)" submits a correct answer$`, w.submitsCorrect)
	ctx.Step(`^"([^"]*)" submits a correct answer and (?:her|his|their) score becomes (\d+)$`, w.submitsCorrectScoreBecomes)
	ctx.Step(`^"([^"]*)" submits a correct answer for question "([^"]*)"$`, w.submitsCorrectFor)
	ctx.Step(`^"([^"]*)" submits an incorrect answer$`, w.submitsIncorrect)

	ctx.Step(`^"([^"]*)"'s score becomes (\d+)$`, w.scoreBecomes)
	ctx.Step(`^the score update is broadcast to all participants$`, w.scoreBroadcast)
	ctx.Step(`^"([^"]*)" receives the score update for "([^"]*)"$`, w.receivesScoreUpdateFor)
	ctx.Step(`^both "([^"]*)" and "([^"]*)" have score (\d+)$`, w.bothHaveScore)
	ctx.Step(`^"([^"]*)" has score (\d+) after answering (\d+) questions correctly$`, w.scoreAfterAnswering)
	ctx.Step(`^a new question "([^"]*)" is broadcast$`, w.currentQuestionIs)
	ctx.Step(`^"([^"]*)"'s score remains (\d+) until she answers "([^"]*)"$`, w.scoreRemainsUntil)
	ctx.Step(`^"([^"]*)"'s score remains (\d+)$`, w.scoreRemains)
}

func (w *World) activePlain(_ string) error {
	if err := w.createSession("manual", 0, 5); err != nil {
		return err
	}
	return w.hostStarts()
}

func (w *World) participantWithScoreStep(name string, score int) error {
	return w.participantWithScore(name, score)
}

func (w *World) connectedStep(name string) error { return w.wsJoin(name) }

func (w *World) bothScoreZero(a, b string) error {
	if err := w.wsJoin(a); err != nil {
		return err
	}
	return w.wsJoin(b)
}

func (w *World) submitsCorrect(name string) error {
	qid, err := w.currentQID()
	if err != nil {
		return err
	}
	_, err = w.wsSubmitAwaitScore(name, qid, correctAnswerFor(qid))
	return err
}

func (w *World) submitsCorrectScoreBecomes(name string, score int) error {
	if err := w.submitsCorrect(name); err != nil {
		return err
	}
	return w.scoreBecomes(name, score)
}

func (w *World) submitsCorrectFor(name, qid string) error {
	_, err := w.wsSubmitAwaitScore(name, qid, correctAnswerFor(qid))
	return err
}

func (w *World) submitsIncorrect(name string) error {
	qid, err := w.currentQID()
	if err != nil {
		return err
	}
	_, err = w.wsSubmitAwaitScore(name, qid, wrongAnswerFor(qid))
	return err
}

func (w *World) scoreBecomes(name string, score int) error {
	if w.lastScoreUpdate.NewScore != score {
		return fmt.Errorf("%s score_update NewScore = %d, want %d", name, w.lastScoreUpdate.NewScore, score)
	}
	return nil
}

func (w *World) scoreBroadcast() error {
	if w.lastScoreUpdate.UserID == "" {
		return fmt.Errorf("no score_update was broadcast")
	}
	return nil
}

func (w *World) receivesScoreUpdateFor(receiver, subject string) error {
	rp := w.participantFor(receiver)
	if rp.ws == nil {
		return fmt.Errorf("%s has no websocket", receiver)
	}
	env, ok := rp.ws.waitFor(msgScoreUpdate, wsTimeout)
	if !ok {
		return fmt.Errorf("%s did not receive a score_update", receiver)
	}
	var sp scoreUpdatePayload
	if err := json.Unmarshal(env.Payload, &sp); err != nil {
		return err
	}
	if sp.UserID != userIDFor(subject) {
		return fmt.Errorf("%s received score_update for %q, want %q", receiver, sp.UserID, userIDFor(subject))
	}
	return nil
}

func (w *World) bothHaveScore(a, b string, score int) error {
	if err := w.assertScore(a, score); err != nil {
		return err
	}
	return w.assertScore(b, score)
}

func (w *World) assertScore(name string, want int) error {
	got, found, err := w.scoreOf(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%s not on leaderboard", name)
	}
	if got != want {
		return fmt.Errorf("%s score = %d, want %d", name, got, want)
	}
	return nil
}

func (w *World) scoreAfterAnswering(name string, score, _ int) error {
	return w.participantWithScore(name, score)
}

func (w *World) scoreRemainsUntil(name string, score int, _ string) error {
	return w.assertScore(name, score)
}

func (w *World) scoreRemains(name string, score int) error {
	return w.assertScore(name, score)
}
