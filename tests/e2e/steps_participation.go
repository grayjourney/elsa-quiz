//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func registerParticipationSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^"([^"]*)" is a participant in session "([^"]*)"$`, w.isParticipantInSession)
	ctx.Step(`^"([^"]*)" is not a participant in session "([^"]*)"$`, func(string, string) error { return nil })
	ctx.Step(`^the current question is:$`, w.currentQuestionDocString)
	ctx.Step(`^the current question has correct answer "([^"]*)" for question "([^"]*)"$`, w.currentQuestionHasAnswer)

	ctx.Step(`^"([^"]*)" submits answer "([^"]*)" for question "([^"]*)"$`, w.submitsAnswerForQ)
	ctx.Step(`^"([^"]*)" submits another answer "([^"]*)" for question "([^"]*)"$`, w.submitsAnswerForQ)
	ctx.Step(`^"([^"]*)" has already submitted answer "([^"]*)" for question "([^"]*)"$`, w.alreadySubmitted)

	ctx.Step(`^the answer is marked as correct$`, w.answerCorrect)
	ctx.Step(`^the answer is marked as incorrect$`, w.answerIncorrect)
	ctx.Step(`^"([^"]*)"'s score is increased by the base points$`, w.scoreIncreasedByBase)
	ctx.Step(`^"([^"]*)"'s score is not increased$`, w.scoreNotIncreased)

	ctx.Step(`^the following participants are connected:$`, w.followingConnected)
	ctx.Step(`^the next question is broadcast$`, w.nextQuestionBroadcast)
	ctx.Step(`^all (\d+) participants receive the question simultaneously$`, w.allReceiveQuestion)
	ctx.Step(`^each participant can submit an answer$`, w.eachCanSubmit)

	ctx.Step(`^the system rejects the duplicate submission$`, w.rejectsDuplicate)
	ctx.Step(`^the system returns an error "([^"]*)"$`, w.systemReturnsError)
	ctx.Step(`^no score is recorded for "([^"]*)"$`, w.noScoreRecorded)
}

func (w *World) isParticipantInSession(name, _ string) error { return w.wsJoin(name) }

func (w *World) currentQuestionDocString(doc *godog.DocString) error {
	var q struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(doc.Content), &q); err != nil {
		return fmt.Errorf("parsing question doc-string: %w", err)
	}
	return w.currentQuestionIs(q.ID)
}

func (w *World) currentQuestionHasAnswer(_, qid string) error { return w.currentQuestionIs(qid) }

func (w *World) submitsAnswerForQ(name, answer, qid string) error {
	return w.submitREST(name, qid, answer)
}

func (w *World) alreadySubmitted(name, answer, qid string) error {
	if err := w.submitREST(name, qid, answer); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("%s first answer status = %d (%s)", name, w.lastStatus, w.lastErr.Code)
	}
	return nil
}

func (w *World) lastSubmit() (submitResp, error) {
	var r submitResp
	if w.lastStatus != 200 {
		return r, fmt.Errorf("submission failed: status %d (%s)", w.lastStatus, w.lastErr.Code)
	}
	return r, w.decodeLast(&r)
}

func (w *World) answerCorrect() error {
	r, err := w.lastSubmit()
	if err != nil {
		return err
	}
	if !r.IsCorrect {
		return fmt.Errorf("answer marked incorrect, want correct")
	}
	return nil
}

func (w *World) answerIncorrect() error {
	r, err := w.lastSubmit()
	if err != nil {
		return err
	}
	if r.IsCorrect {
		return fmt.Errorf("answer marked correct, want incorrect")
	}
	return nil
}

func (w *World) scoreIncreasedByBase(_ string) error {
	r, err := w.lastSubmit()
	if err != nil {
		return err
	}
	if r.AwardedPoints != 10 {
		return fmt.Errorf("awarded points = %d, want 10 (base)", r.AwardedPoints)
	}
	return nil
}

func (w *World) scoreNotIncreased(_ string) error {
	r, err := w.lastSubmit()
	if err != nil {
		return err
	}
	if r.AwardedPoints != 0 {
		return fmt.Errorf("awarded points = %d, want 0", r.AwardedPoints)
	}
	return nil
}

func (w *World) followingConnected(doc *godog.DocString) error {
	w.connected = w.connected[:0]
	for _, line := range strings.Split(strings.TrimSpace(doc.Content), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		p := w.participantFor(name)
		if p.ws == nil {
			if err := w.wsJoin(name); err != nil {
				return err
			}
		}
		w.connected = append(w.connected, name)
	}
	return nil
}

func (w *World) nextQuestionBroadcast() error {
	if err := w.advanceREST(); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("advance status = %d (%s)", w.lastStatus, w.lastErr.Code)
	}
	qid, err := w.currentQID()
	if err != nil {
		return err
	}
	w.expectedQID = qid
	return nil
}

func (w *World) allReceiveQuestion(_ int) error {
	for _, name := range w.connected {
		if err := w.waitForQuestionID(name, w.expectedQID); err != nil {
			return err
		}
	}
	return nil
}

func (w *World) eachCanSubmit() error {
	for _, name := range w.connected {
		if err := w.submitREST(name, w.expectedQID, correctAnswerFor(w.expectedQID)); err != nil {
			return err
		}
		if w.lastStatus != 200 {
			return fmt.Errorf("%s submit status = %d (%s)", name, w.lastStatus, w.lastErr.Code)
		}
	}
	return nil
}

func (w *World) rejectsDuplicate() error {
	if w.lastStatus == 200 {
		return fmt.Errorf("duplicate submission was accepted (status 200)")
	}
	return nil
}

func (w *World) systemReturnsError(message string) error {
	if w.lastErr.Message != message {
		return fmt.Errorf("error message = %q (code %s), want %q", w.lastErr.Message, w.lastErr.Code, message)
	}
	return nil
}

func (w *World) noScoreRecorded(name string) error {
	_, found, err := w.scoreOf(name)
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("a score was recorded for %s but none should exist", name)
	}
	return nil
}
