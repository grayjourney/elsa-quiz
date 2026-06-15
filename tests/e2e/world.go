//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// World is the per-scenario state. godog calls the ScenarioInitializer once per
// scenario, so each scenario gets a fresh World (isolation by construction).
type World struct {
	rest   *restClient
	wsBase string

	quizID           string
	timeLimitSeconds int
	participants     map[string]*participant
	connected        []string // names joined via "N participants are connected"
	lastCreate       createSessionResp
	lastScoreUpdate  scoreUpdatePayload
	expectedQID      string
	perfSessions     []perfSession
	latencies        []time.Duration
	perfAccepted     int
	perfTotal        int

	// last REST interaction
	lastStatus int
	lastRaw    []byte
	lastErr    errorResp
}

type participant struct {
	name           string
	userID         string
	displayName    string
	ws             *wsClient
	joined         bool
	joinScore      int    // score reported in join_confirmed
	knownScore     int    // last score this participant was observed to have
	lastQuestionID string // id of the most recent question this client received
}

const wsTimeout = 3 * time.Second

func newWorld() *World {
	base := envOr("E2E_BASE_URL", "http://localhost:8080")
	return &World{
		rest:         newREST(base),
		wsBase:       envOr("E2E_WS_URL", "ws://localhost:8080"),
		participants: map[string]*participant{},
	}
}

func (w *World) close() {
	for _, p := range w.participants {
		if p.ws != nil {
			p.ws.close()
		}
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func userIDFor(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func (w *World) participantFor(name string) *participant {
	id := userIDFor(name)
	if p, ok := w.participants[id]; ok {
		return p
	}
	p := &participant{name: name, userID: id, displayName: name}
	w.participants[id] = p
	return p
}

// ---- REST plumbing ----

func (w *World) req(method, path, userID string, body any) error {
	st, raw, err := w.rest.do(method, path, userID, body)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	w.lastStatus, w.lastRaw, w.lastErr = st, raw, errorResp{}
	if st >= 400 {
		_ = json.Unmarshal(raw, &w.lastErr)
	}
	return nil
}

func (w *World) decodeLast(dst any) error {
	if err := json.Unmarshal(w.lastRaw, dst); err != nil {
		return fmt.Errorf("decoding response %q: %w", string(w.lastRaw), err)
	}
	return nil
}

// ---- domain test data ----

func defaultQuestions(n int) []questionReq {
	all := []questionReq{
		{ID: "Q1", Text: "past tense of 'go'", Options: []string{"goed", "went", "gone", "going"}, CorrectAnswer: "went", Order: 1},
		{ID: "Q2", Text: "plural of 'mouse'", Options: []string{"mouses", "mice", "meese", "mouse"}, CorrectAnswer: "mice", Order: 2},
		{ID: "Q3", Text: "opposite of 'hot'", Options: []string{"warm", "cold", "cool", "tepid"}, CorrectAnswer: "cold", Order: 3},
		{ID: "Q4", Text: "a synonym for 'happy'", Options: []string{"sad", "glad", "mad", "bad"}, CorrectAnswer: "glad", Order: 4},
		{ID: "Q5", Text: "past tense of 'eat'", Options: []string{"eated", "ate", "eaten", "eating"}, CorrectAnswer: "ate", Order: 5},
	}
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// ---- session helpers ----

func (w *World) createSession(policy string, limitSeconds, questionCount int) error {
	body := createSessionReq{EndPolicy: policy, TimeLimitSeconds: limitSeconds, Questions: defaultQuestions(questionCount)}
	if err := w.req("POST", "/api/sessions", "", body); err != nil {
		return err
	}
	if w.lastStatus != 201 {
		return nil // leave it for the scenario to assert (negative cases)
	}
	var resp createSessionResp
	if err := w.decodeLast(&resp); err != nil {
		return err
	}
	w.lastCreate = resp
	w.quizID = resp.QuizID
	w.timeLimitSeconds = limitSeconds
	return nil
}

func (w *World) leaderboard() ([]lbEntry, error) {
	st, raw, err := w.rawGet("/api/sessions/" + w.quizID + "/leaderboard")
	if err != nil {
		return nil, err
	}
	if st != 200 {
		return nil, fmt.Errorf("get leaderboard status = %d", st)
	}
	var lb leaderboardResp
	if err := json.Unmarshal(raw, &lb); err != nil {
		return nil, err
	}
	return lb.Leaderboard, nil
}

type perfSession struct {
	id    string
	users []string
}

// wsSubmitMeasure sends a correct answer over name's socket and returns the
// round-trip latency until that participant's own score_update arrives.
func (w *World) wsSubmitMeasure(name string) (time.Duration, error) {
	p := w.participantFor(name)
	if p.ws == nil {
		return 0, fmt.Errorf("%s has no websocket", name)
	}
	qid, err := w.currentQID()
	if err != nil {
		return 0, err
	}
	start := time.Now()
	if err := p.ws.send(qid, correctAnswerFor(qid)); err != nil {
		return 0, err
	}
	if _, ok := p.ws.waitFor(msgScoreUpdate, wsTimeout); !ok {
		return 0, fmt.Errorf("%s did not receive its score_update", name)
	}
	return time.Since(start), nil
}

// createManySessions builds n sessions, each with m REST-joined participants,
// all started — for the throughput smoke test.
func (w *World) createManySessions(n, m int) error {
	w.perfSessions = nil
	for i := range n {
		if err := w.createSession("manual", 0, 1); err != nil {
			return err
		}
		sess := perfSession{id: w.quizID}
		for j := range m {
			uid := fmt.Sprintf("s%d-u%d", i, j)
			st, _, err := w.rest.do("POST", "/api/sessions/"+sess.id+"/participants", "", joinReq{UserID: uid, DisplayName: uid})
			if err != nil || st != 201 {
				return fmt.Errorf("join %s status=%d err=%v", uid, st, err)
			}
			sess.users = append(sess.users, uid)
		}
		if _, _, err := w.rest.do("POST", "/api/sessions/"+sess.id+"/start", "host", nil); err != nil {
			return err
		}
		w.perfSessions = append(w.perfSessions, sess)
	}
	return nil
}

func (w *World) sumScores() (int, error) {
	board, err := w.leaderboard()
	if err != nil {
		return 0, err
	}
	sum := 0
	for _, e := range board {
		sum += e.Score
	}
	return sum, nil
}

// concurrentSubmit fires answer submissions for all names at once over REST.
// It bypasses the shared last-action fields (thread-safety) and reports the
// number of accepted (HTTP 200) submissions.
func (w *World) concurrentSubmit(names []string, qid, answer string) int {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
		ok int
	)
	for _, name := range names {
		uid := userIDFor(name)
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			st, _, err := w.rest.do("POST", "/api/sessions/"+w.quizID+"/answers", uid, submitReq{QuestionID: qid, Answer: answer})
			if err == nil && st == 200 {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(uid)
	}
	wg.Wait()
	return ok
}

func (w *World) scoreOf(name string) (int, bool, error) {
	board, err := w.leaderboard()
	if err != nil {
		return 0, false, err
	}
	for _, e := range board {
		if e.UserID == userIDFor(name) {
			return e.Score, true, nil
		}
	}
	return 0, false, nil
}

// rawGet performs a GET without touching the last-action fields (lastStatus/
// lastRaw/lastErr), so internal reads inside assertion steps never clobber the
// error/status produced by the step's actual action.
func (w *World) rawGet(path string) (int, []byte, error) {
	return w.rest.do("GET", path, "", nil)
}

func (w *World) getSession() (getSessionResp, error) {
	st, raw, err := w.rawGet("/api/sessions/" + w.quizID)
	if err != nil {
		return getSessionResp{}, err
	}
	if st != 200 {
		return getSessionResp{}, fmt.Errorf("get session status = %d", st)
	}
	var s getSessionResp
	if err := json.Unmarshal(raw, &s); err != nil {
		return getSessionResp{}, err
	}
	return s, nil
}

func (w *World) getStatus() (string, error) {
	s, err := w.getSession()
	return s.Status, err
}

func (w *World) joinREST(name string) error {
	p := w.participantFor(name)
	if err := w.req("POST", "/api/sessions/"+w.quizID+"/participants", "", joinReq{UserID: p.userID, DisplayName: p.displayName}); err != nil {
		return err
	}
	if w.lastStatus == 201 {
		p.joined = true
	}
	return nil
}

func (w *World) startREST() error {
	return w.req("POST", "/api/sessions/"+w.quizID+"/start", "host", nil)
}

func (w *World) advanceREST() error {
	return w.req("POST", "/api/sessions/"+w.quizID+"/advance", "host", nil)
}

func (w *World) endREST() error {
	return w.req("POST", "/api/sessions/"+w.quizID+"/end", "host", nil)
}

func (w *World) submitREST(name, questionID, answer string) error {
	p := w.participantFor(name)
	if err := w.req("POST", "/api/sessions/"+w.quizID+"/answers", p.userID, submitReq{QuestionID: questionID, Answer: answer}); err != nil {
		return err
	}
	if w.lastStatus == 200 {
		var resp submitResp
		if json.Unmarshal(w.lastRaw, &resp) == nil {
			p.knownScore = resp.NewScore
		}
	}
	return nil
}

// resolveQuizID maps a scenario's literal quiz-ID label to the real one. Server
// IDs are generated, so a session created in the scenario wins; an empty label
// stays empty (to exercise quiz_id_required); otherwise the literal is used as a
// genuinely non-existent id.
func (w *World) resolveQuizID(label string) string {
	if label == "" {
		return ""
	}
	if w.quizID != "" {
		return w.quizID
	}
	return label
}

func correctAnswerFor(qid string) string {
	for _, q := range defaultQuestions(5) {
		if q.ID == qid {
			return q.CorrectAnswer
		}
	}
	return ""
}

func wrongAnswerFor(qid string) string {
	for _, q := range defaultQuestions(5) {
		if q.ID == qid {
			for _, opt := range q.Options {
				if opt != q.CorrectAnswer {
					return opt
				}
			}
		}
	}
	return "definitely-wrong"
}

func (w *World) currentQID() (string, error) {
	s, err := w.getSession()
	if err != nil {
		return "", err
	}
	if s.CurrentQuestion == nil {
		return "", fmt.Errorf("no current question")
	}
	return s.CurrentQuestion.ID, nil
}

// participantWithScore brings name to the given score by answering that many
// questions correctly over REST (manual policy), advancing after each so the
// next answer targets a fresh question. Leaves a fresh current question.
func (w *World) participantWithScore(name string, score int) error {
	if err := w.wsJoin(name); err != nil {
		return err
	}
	for range score / 10 {
		qid, err := w.currentQID()
		if err != nil {
			return err
		}
		if err := w.submitREST(name, qid, correctAnswerFor(qid)); err != nil {
			return err
		}
		if w.lastStatus != 200 {
			return fmt.Errorf("%s preset answer status = %d (%s)", name, w.lastStatus, w.lastErr.Code)
		}
		if err := w.advanceREST(); err != nil {
			return err
		}
	}
	return nil
}

// wsSubmitAwaitScore sends a submit_answer over name's socket and returns the
// score_update the server broadcasts back (the submitter receives it too).
func (w *World) wsSubmitAwaitScore(name, qid, answer string) (scoreUpdatePayload, error) {
	p := w.participantFor(name)
	if p.ws == nil {
		return scoreUpdatePayload{}, fmt.Errorf("%s has no websocket", name)
	}
	if err := p.ws.send(qid, answer); err != nil {
		return scoreUpdatePayload{}, err
	}
	env, ok := p.ws.waitFor(msgScoreUpdate, wsTimeout)
	if !ok {
		return scoreUpdatePayload{}, fmt.Errorf("%s did not receive a score_update", name)
	}
	var sp scoreUpdatePayload
	if err := json.Unmarshal(env.Payload, &sp); err != nil {
		return scoreUpdatePayload{}, err
	}
	w.lastScoreUpdate = sp
	p.knownScore = sp.NewScore
	return sp, nil
}

// waitForQuestionID drains question messages on name's socket until one with the
// given id arrives (so an earlier question already buffered is skipped).
func (w *World) waitForQuestionID(name, id string) error {
	p := w.participantFor(name)
	if p.ws == nil {
		return fmt.Errorf("%s has no websocket", name)
	}
	deadline := time.Now().Add(wsTimeout)
	for time.Now().Before(deadline) {
		env, ok := p.ws.waitFor(msgQuestion, wsTimeout)
		if !ok {
			break
		}
		var q questionPayload
		if err := json.Unmarshal(env.Payload, &q); err != nil {
			return err
		}
		if q.ID == id {
			p.lastQuestionID = q.ID
			return nil
		}
	}
	return fmt.Errorf("%s did not receive question %q", name, id)
}

// wsJoin opens a WebSocket for name and consumes the initial join_confirmed,
// stashing the reported score. The connection stays open for later assertions.
func (w *World) wsJoin(name string) error {
	p := w.participantFor(name)
	c, _, err := dialWS(w.wsBase, w.quizID, p.userID, p.displayName)
	if err != nil {
		return fmt.Errorf("ws dial for %s: %w", name, err)
	}
	p.ws = c
	env, ok := c.waitFor(msgJoinConfirmed, wsTimeout)
	if !ok {
		return fmt.Errorf("%s did not receive join_confirmed", name)
	}
	var jc joinConfirmedPayload
	if err := json.Unmarshal(env.Payload, &jc); err != nil {
		return err
	}
	p.joinScore = jc.Score
	p.joined = true
	return nil
}

// expectQuestion waits for a question on name's socket and records its id.
func (w *World) expectQuestion(name string) (questionPayload, error) {
	p := w.participantFor(name)
	if p.ws == nil {
		return questionPayload{}, fmt.Errorf("%s has no websocket", name)
	}
	env, ok := p.ws.waitFor(msgQuestion, wsTimeout)
	if !ok {
		return questionPayload{}, fmt.Errorf("%s did not receive a question", name)
	}
	var q questionPayload
	if err := json.Unmarshal(env.Payload, &q); err != nil {
		return questionPayload{}, err
	}
	p.lastQuestionID = q.ID
	return q, nil
}
