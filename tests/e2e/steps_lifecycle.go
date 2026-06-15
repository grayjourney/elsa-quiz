//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func registerLifecycleSteps(ctx *godog.ScenarioContext, w *World) {
	// creation / policy
	ctx.Step(`^a host creates a new quiz session with end policy "([^"]*)"$`, w.createWithPolicy)
	ctx.Step(`^a host creates a new quiz session with end policy "([^"]*)" and a per-question time limit of (\d+) seconds?$`, w.createWithTimed)
	ctx.Step(`^the session is created with end policy "([^"]*)"$`, w.createdWithPolicy)
	ctx.Step(`^each question carries a time limit of (\d+) seconds?$`, w.questionTimeLimitIs)
	ctx.Step(`^the request is rejected with error code "([^"]*)"$`, w.requestRejected)
	ctx.Step(`^no session is created$`, w.noSessionCreated)

	// start
	ctx.Step(`^a quiz session "([^"]*)" exists with status "([^"]*)"$`, w.sessionExistsWithStatus)
	ctx.Step(`^(\d+) participants have joined$`, w.nParticipantsJoined)
	ctx.Step(`^the host starts the quiz$`, w.hostStarts)
	ctx.Step(`^the session status changes to "([^"]*)"$`, w.assertStatus)
	ctx.Step(`^the first question is broadcast to all participants$`, w.firstQuestionBroadcast)

	// active preconditions
	ctx.Step(`^a quiz session "([^"]*)" is active with end policy "([^"]*)"$`, w.activeWithPolicy)
	ctx.Step(`^a quiz session "([^"]*)" is active with end policy "([^"]*)" and a (\d+) second time limit$`, w.activeWithTimed)
	ctx.Step(`^a quiz session "([^"]*)" has status "completed"$`, w.sessionCompletedSetup)

	// manual advance / end
	ctx.Step(`^the current question is "([^"]*)"$`, w.currentQuestionIs)
	ctx.Step(`^the current question is the final question "([^"]*)"$`, w.currentQuestionIsFinal)
	ctx.Step(`^the host advances to the next question$`, w.hostAdvances)
	ctx.Step(`^the host advances past "([^"]*)"$`, w.hostAdvancesPast)
	ctx.Step(`^question "([^"]*)" is broadcast to all participants$`, w.questionBroadcast)
	ctx.Step(`^question "([^"]*)" is broadcast without waiting for the timer$`, w.questionBroadcast)
	ctx.Step(`^no further answers are accepted for "([^"]*)"$`, w.noFurtherAnswersFor)
	ctx.Step(`^the host ends the quiz$`, w.hostEnds)
	ctx.Step(`^the host ends the quiz again$`, w.hostEnds)
	ctx.Step(`^the session remains "([^"]*)"$`, w.assertStatus)

	// participants table + AFK
	ctx.Step(`^the participants are:$`, w.participantsAre)
	ctx.Step(`^the final leaderboard is displayed to all participants$`, w.finalLeaderboardDisplayed)
	ctx.Step(`^"([^"]*)" is scored 0 for the unanswered question$`, w.scoredZero)
	ctx.Step(`^"([^"]*)" is scored 0 for "([^"]*)"$`, w.scoredZeroFor)
	ctx.Step(`^no further answer submissions are accepted$`, w.noFurtherSubmissions)

	// timed
	ctx.Step(`^the current question "([^"]*)" has been open$`, func(string) error { return nil })
	ctx.Step(`^"([^"]*)" has not answered "([^"]*)"$`, w.hasNotAnswered)
	ctx.Step(`^the time limit for "([^"]*)" expires$`, w.timeLimitExpires)
	ctx.Step(`^the time limit for question "([^"]*)" has expired$`, w.timeLimitExpires)
	ctx.Step(`^(\d+) participants are connected$`, w.nConnected)
	ctx.Step(`^the current question is the final question$`, func() error { return nil })
	ctx.Step(`^all (\d+) connected participants submit an answer for "([^"]*)" before the time limit$`, w.allConnectedSubmit)
	ctx.Step(`^the quiz has (\d+) questions total$`, w.quizHasQuestions)
	ctx.Step(`^"([^"]*)" goes AFK after question 1$`, w.goesAFK)
	ctx.Step(`^the time limit expires for each remaining question$`, w.allRemainingExpire)

	// end guards / @timing
	ctx.Step(`^"([^"]*)" is a participant$`, w.isAParticipant)
	ctx.Step(`^"([^"]*)" submits an answer for "([^"]*)"$`, w.submitsAnswerFor)
	ctx.Step(`^"([^"]*)" tries to submit an answer$`, w.triesToSubmit)
	ctx.Step(`^"([^"]*)"'s score remains unchanged$`, w.scoreUnchanged)
}

func (w *World) createWithPolicy(policy string) error { return w.createSession(policy, 0, 3) }

func (w *World) createWithTimed(policy string, sec int) error { return w.createSession(policy, sec, 3) }

func (w *World) createdWithPolicy(policy string) error {
	if w.lastCreate.EndPolicy != policy {
		return fmt.Errorf("created end policy = %q, want %q", w.lastCreate.EndPolicy, policy)
	}
	return nil
}

func (w *World) questionTimeLimitIs(sec int) error {
	if w.lastCreate.TimeLimitSeconds != sec {
		return fmt.Errorf("time limit = %ds, want %ds", w.lastCreate.TimeLimitSeconds, sec)
	}
	return nil
}

func (w *World) requestRejected(code string) error {
	if w.lastErr.Code != code {
		return fmt.Errorf("error code = %q (status %d), want %q", w.lastErr.Code, w.lastStatus, code)
	}
	return nil
}

func (w *World) noSessionCreated() error {
	if w.quizID != "" {
		return fmt.Errorf("a session was created (%s) but should not have been", w.quizID)
	}
	return nil
}

func (w *World) sessionExistsWithStatus(_, status string) error {
	if err := w.createSession("manual", 0, 3); err != nil {
		return err
	}
	return w.ensureSessionStatus(status)
}

func (w *World) nParticipantsJoined(n int) error {
	for i := 1; i <= n; i++ {
		if err := w.joinREST(fmt.Sprintf("u%d", i)); err != nil {
			return err
		}
		if w.lastStatus != 201 {
			return fmt.Errorf("join u%d status = %d (%s)", i, w.lastStatus, w.lastErr.Code)
		}
	}
	return nil
}

func (w *World) hostStarts() error {
	if err := w.startREST(); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("start status = %d (%s)", w.lastStatus, w.lastErr.Code)
	}
	return nil
}

func (w *World) assertStatus(status string) error {
	cur, err := w.getStatus()
	if err != nil {
		return err
	}
	if cur != status {
		return fmt.Errorf("status = %q, want %q", cur, status)
	}
	return nil
}

func (w *World) firstQuestionBroadcast() error {
	s, err := w.getSession()
	if err != nil {
		return err
	}
	if s.CurrentQuestion == nil {
		return fmt.Errorf("no current question after start")
	}
	return nil
}

func (w *World) activeWithPolicy(_, policy string) error {
	if err := w.createSession(policy, 0, 3); err != nil {
		return err
	}
	return w.hostStarts()
}

func (w *World) activeWithTimed(_, policy string, sec int) error {
	if err := w.createSession(policy, sec, 3); err != nil {
		return err
	}
	return w.hostStarts()
}

func (w *World) sessionCompletedSetup(_ string) error {
	if err := w.createSession("manual", 0, 3); err != nil {
		return err
	}
	if err := w.joinREST("Alice"); err != nil {
		return err
	}
	if err := w.hostStarts(); err != nil {
		return err
	}
	if err := w.endREST(); err != nil {
		return err
	}
	return w.assertStatus("completed")
}

func (w *World) currentQuestionIs(id string) error {
	s, err := w.getSession()
	if err != nil {
		return err
	}
	if s.CurrentQuestion == nil || s.CurrentQuestion.ID != id {
		return fmt.Errorf("current question = %v, want %q", s.CurrentQuestion, id)
	}
	return nil
}

func (w *World) currentQuestionIsFinal(id string) error {
	for range 10 {
		s, err := w.getSession()
		if err != nil {
			return err
		}
		if s.CurrentQuestion != nil && s.CurrentQuestion.ID == id {
			return nil
		}
		if err := w.advanceREST(); err != nil {
			return err
		}
		if w.lastStatus != 200 {
			return fmt.Errorf("advance status = %d (%s)", w.lastStatus, w.lastErr.Code)
		}
	}
	return fmt.Errorf("did not reach final question %q", id)
}

func (w *World) hostAdvances() error {
	if err := w.advanceREST(); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("advance status = %d (%s)", w.lastStatus, w.lastErr.Code)
	}
	return nil
}

func (w *World) hostAdvancesPast(_ string) error { return w.hostAdvances() }

func (w *World) questionBroadcast(id string) error { return w.currentQuestionIs(id) }

func (w *World) noFurtherAnswersFor(qid string) error {
	if err := w.joinREST("Tester"); err != nil {
		return err
	}
	if err := w.submitREST("Tester", qid, correctAnswerFor(qid)); err != nil {
		return err
	}
	if w.lastStatus == 200 {
		return fmt.Errorf("answer for closed question %q was accepted (status 200)", qid)
	}
	return nil
}

func (w *World) hostEnds() error { return w.endREST() }

func (w *World) participantsAre(table *godog.DocString) error {
	for _, line := range strings.Split(strings.TrimSpace(table.Content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := strings.Fields(line)[0]
		answered := strings.Contains(strings.ToLower(line), "answered")
		if err := w.wsJoin(name); err != nil {
			return err
		}
		if answered {
			if err := w.submitREST(name, "Q1", correctAnswerFor("Q1")); err != nil {
				return err
			}
			if w.lastStatus != 200 {
				return fmt.Errorf("%s answer status = %d (%s)", name, w.lastStatus, w.lastErr.Code)
			}
		}
	}
	return nil
}

func (w *World) finalLeaderboardDisplayed() error {
	var withWS []*participant
	for _, p := range w.participants {
		if p.ws != nil {
			withWS = append(withWS, p)
		}
	}
	if len(withWS) == 0 {
		if _, err := w.leaderboard(); err != nil {
			return fmt.Errorf("final leaderboard not retrievable: %w", err)
		}
		return nil
	}
	for _, p := range withWS {
		if _, ok := p.ws.waitFor(msgQuizEnded, wsTimeout); !ok {
			return fmt.Errorf("%s did not receive quiz_ended", p.name)
		}
	}
	return nil
}

func (w *World) scoredZero(name string) error { return w.scoredZeroFor(name, "") }

func (w *World) scoredZeroFor(name, _ string) error {
	score, found, err := w.scoreOf(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%s not on the leaderboard", name)
	}
	if score != 0 {
		return fmt.Errorf("%s score = %d, want 0", name, score)
	}
	return nil
}

func (w *World) noFurtherSubmissions() error {
	if err := w.submitREST("Alice", "Q1", correctAnswerFor("Q1")); err != nil {
		return err
	}
	if w.lastStatus == 200 {
		return fmt.Errorf("submission after end was accepted (status 200)")
	}
	return nil
}

func (w *World) hasNotAnswered(name, _ string) error { return w.wsJoin(name) }

func (w *World) timeLimitExpires(_ string) error {
	time.Sleep(time.Duration(w.timeLimitSeconds)*time.Second + 500*time.Millisecond)
	return nil
}

func (w *World) nConnected(n int) error {
	w.connected = w.connected[:0]
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("P%d", i)
		if err := w.wsJoin(name); err != nil {
			return err
		}
		w.connected = append(w.connected, name)
	}
	return nil
}

func (w *World) allConnectedSubmit(_ int, qid string) error {
	for _, name := range w.connected {
		if err := w.submitREST(name, qid, correctAnswerFor(qid)); err != nil {
			return err
		}
		if w.lastStatus != 200 {
			return fmt.Errorf("%s submit status = %d (%s)", name, w.lastStatus, w.lastErr.Code)
		}
	}
	// give the server a moment to fire the early-advance broadcast
	time.Sleep(150 * time.Millisecond)
	return nil
}

func (w *World) quizHasQuestions(n int) error {
	if w.lastCreate.QuestionCount != n {
		return fmt.Errorf("question count = %d, want %d", w.lastCreate.QuestionCount, n)
	}
	return nil
}

func (w *World) goesAFK(name string) error { return w.wsJoin(name) }

func (w *World) allRemainingExpire() error {
	// sleep long enough for every question's timer to fire and the quiz to complete
	total := time.Duration(w.lastCreate.QuestionCount+1)*time.Duration(w.timeLimitSeconds)*time.Second + 500*time.Millisecond
	time.Sleep(total)
	return nil
}

func (w *World) isAParticipant(name string) error {
	p := w.participantFor(name)
	if p.joined {
		return nil
	}
	if err := w.joinREST(name); err != nil {
		return err
	}
	if w.lastStatus != 201 {
		return fmt.Errorf("%s could not join (status %d, %s)", name, w.lastStatus, w.lastErr.Code)
	}
	return nil
}

func (w *World) submitsAnswerFor(name, qid string) error {
	return w.submitREST(name, qid, correctAnswerFor(qid))
}

func (w *World) triesToSubmit(name string) error {
	return w.submitREST(name, "Q1", correctAnswerFor("Q1"))
}

func (w *World) scoreUnchanged(name string) error {
	p := w.participantFor(name)
	score, _, err := w.scoreOf(name)
	if err != nil {
		return err
	}
	if score != p.knownScore {
		return fmt.Errorf("%s score = %d, want %d (unchanged)", name, score, p.knownScore)
	}
	return nil
}
