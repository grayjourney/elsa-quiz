//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func registerSessionSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^a host requests to create a new quiz session$`, w.hostCreatesSession)
	ctx.Step(`^a quiz session exists with ID "([^"]*)"$`, w.sessionExistsWithID)
	ctx.Step(`^no quiz session exists with ID "([^"]*)"$`, func(string) error { return nil })
	ctx.Step(`^the system generates a unique quiz ID$`, w.uniqueQuizID)
	ctx.Step(`^the session status is "([^"]*)"$`, w.ensureSessionStatus)
	ctx.Step(`^the session is available for users to join$`, w.sessionAvailableToJoin)

	ctx.Step(`^a user "([^"]*)" joins the session with ID "([^"]*)"$`, w.userJoins)
	ctx.Step(`^"([^"]*)" joins the session with ID "([^"]*)"$`, w.userJoins)
	ctx.Step(`^"([^"]*)" is already a participant in session "([^"]*)"$`, w.alreadyParticipant)
	ctx.Step(`^the following users join session "([^"]*)" simultaneously:$`, w.usersJoinSimultaneously)

	ctx.Step(`^"([^"]*)" is added to the session's participant list$`, w.isAddedToList)
	ctx.Step(`^all (\d+) users are added to the session's participant list$`, w.allAddedToList)
	ctx.Step(`^"([^"]*)" receives a confirmation of successful join$`, w.receivedConfirmation)
	ctx.Step(`^each user receives a confirmation of successful join$`, w.eachReceivedConfirmation)
	ctx.Step(`^"([^"]*)"'s initial score is 0$`, w.initialScoreZero)
	ctx.Step(`^the session has exactly (\d+) participants$`, w.sessionHasParticipants)

	ctx.Step(`^"([^"]*)" receives a notification that "([^"]*)" has joined$`, w.receivesJoinNotification)
	ctx.Step(`^the participant count is updated to (\d+)$`, w.participantCountIs)

	ctx.Step(`^(\d+) questions have already been broadcast$`, w.questionsBroadcast)
	ctx.Step(`^"([^"]*)" receives the current question$`, w.receivesCurrentQuestion)
	ctx.Step(`^"([^"]*)" can submit answers from the current question onward$`, w.canSubmitFromCurrent)

	ctx.Step(`^a user "([^"]*)" tries to join session "([^"]*)"$`, w.triesToJoin)
	ctx.Step(`^the join is rejected with error code "([^"]*)"$`, w.joinRejected)
	ctx.Step(`^"([^"]*)" is not added to any session$`, w.notAdded)
}

func (w *World) hostCreatesSession() error { return w.createSession("manual", 0, 3) }

func (w *World) sessionExistsWithID(_ string) error { return w.createSession("manual", 0, 3) }

func (w *World) uniqueQuizID() error {
	if w.quizID == "" {
		return fmt.Errorf("no quiz ID was generated")
	}
	if !strings.HasPrefix(w.quizID, "QUIZ-") {
		return fmt.Errorf("quiz ID %q does not look generated (want QUIZ- prefix)", w.quizID)
	}
	return nil
}

func (w *World) ensureSessionStatus(status string) error {
	cur, err := w.getStatus()
	if err != nil {
		return err
	}
	switch status {
	case "active":
		if cur == "waiting" {
			if err := w.startREST(); err != nil {
				return err
			}
		}
	case "completed":
		if cur != "completed" {
			if err := w.endREST(); err != nil {
				return err
			}
		}
	}
	cur, err = w.getStatus()
	if err != nil {
		return err
	}
	if cur != status {
		return fmt.Errorf("session status = %q, want %q", cur, status)
	}
	return nil
}

func (w *World) sessionAvailableToJoin() error {
	if err := w.joinREST("Probe"); err != nil {
		return err
	}
	if w.lastStatus != 201 {
		return fmt.Errorf("session not available to join: status %d (%s)", w.lastStatus, w.lastErr.Code)
	}
	return nil
}

func (w *World) userJoins(name, _ string) error { return w.wsJoin(name) }

func (w *World) alreadyParticipant(name, _ string) error { return w.wsJoin(name) }

func (w *World) usersJoinSimultaneously(_ string, names *godog.DocString) error {
	for _, line := range strings.Split(strings.TrimSpace(names.Content), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if err := w.wsJoin(name); err != nil {
			return err
		}
	}
	return nil
}

func (w *World) isAddedToList(name string) error {
	p := w.participantFor(name)
	if p.ws == nil {
		return fmt.Errorf("%s did not join (no websocket)", name)
	}
	return nil
}

func (w *World) allAddedToList(n int) error { return w.sessionHasParticipants(n) }

func (w *World) receivedConfirmation(name string) error { return w.isAddedToList(name) }

func (w *World) eachReceivedConfirmation() error {
	for _, p := range w.participants {
		if p.name == "Probe" {
			continue
		}
		if p.ws == nil {
			return fmt.Errorf("%s has no join confirmation", p.name)
		}
	}
	return nil
}

func (w *World) initialScoreZero(name string) error {
	p := w.participantFor(name)
	if p.joinScore != 0 {
		return fmt.Errorf("%s initial score = %d, want 0", name, p.joinScore)
	}
	return nil
}

func (w *World) sessionHasParticipants(n int) error {
	s, err := w.getSession()
	if err != nil {
		return err
	}
	if s.ParticipantCount != n {
		return fmt.Errorf("participant count = %d, want %d", s.ParticipantCount, n)
	}
	return nil
}

func (w *World) receivesJoinNotification(notifiee, joiner string) error {
	np := w.participantFor(notifiee)
	if np.ws == nil {
		return fmt.Errorf("%s has no websocket", notifiee)
	}
	env, ok := np.ws.waitFor(msgUserJoined, wsTimeout)
	if !ok {
		return fmt.Errorf("%s did not receive a user_joined notification", notifiee)
	}
	var uj userJoinedPayload
	if err := json.Unmarshal(env.Payload, &uj); err != nil {
		return err
	}
	if uj.UserID != userIDFor(joiner) {
		return fmt.Errorf("user_joined for %q, want %q", uj.UserID, userIDFor(joiner))
	}
	return nil
}

func (w *World) participantCountIs(n int) error { return w.sessionHasParticipants(n) }

func (w *World) questionsBroadcast(n int) error {
	if err := w.ensureSessionStatus("active"); err != nil {
		return err
	}
	for i := 1; i < n; i++ { // start already broadcast the first
		if err := w.advanceREST(); err != nil {
			return err
		}
		if w.lastStatus != 200 {
			return fmt.Errorf("advance status = %d (%s)", w.lastStatus, w.lastErr.Code)
		}
	}
	return nil
}

func (w *World) receivesCurrentQuestion(name string) error {
	_, err := w.expectQuestion(name)
	return err
}

func (w *World) canSubmitFromCurrent(name string) error {
	p := w.participantFor(name)
	if p.lastQuestionID == "" {
		if _, err := w.expectQuestion(name); err != nil {
			return err
		}
	}
	answer := correctAnswerFor(p.lastQuestionID)
	if err := w.submitREST(name, p.lastQuestionID, answer); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("%s submit status = %d (%s)", name, w.lastStatus, w.lastErr.Code)
	}
	var resp submitResp
	if err := w.decodeLast(&resp); err != nil {
		return err
	}
	if !resp.IsCorrect {
		return fmt.Errorf("%s answer to %s not accepted as correct", name, p.lastQuestionID)
	}
	return nil
}

func (w *World) triesToJoin(name, label string) error {
	p := w.participantFor(name)
	status, code := tryDialWS(w.wsBase, w.resolveQuizID(label), p.userID, p.displayName)
	w.lastStatus, w.lastErr = status, errorResp{Code: code}
	return nil
}

func (w *World) joinRejected(code string) error {
	if w.lastErr.Code != code {
		return fmt.Errorf("join error code = %q (status %d), want %q", w.lastErr.Code, w.lastStatus, code)
	}
	return nil
}

func (w *World) notAdded(name string) error {
	p := w.participantFor(name)
	if p.ws != nil {
		return fmt.Errorf("%s was added but should not have been", name)
	}
	return nil
}
