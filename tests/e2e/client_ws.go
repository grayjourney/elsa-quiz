//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsClient is a black-box WebSocket client. A background reader funnels every
// inbound envelope into a channel; waitFor consumes by type, stashing other
// types so nothing is lost (presence semantics, order-independent).
type wsClient struct {
	conn    *websocket.Conn
	raw     chan wsEnvelope
	mu      sync.Mutex
	pending []wsEnvelope
}

// dialWS opens /ws with the given identity. The second return is the HTTP
// status of a *failed* handshake (the server validates+joins before upgrading),
// so a rejected join surfaces its status code here.
func dialWS(wsBase, quizID, userID, name string) (*wsClient, int, error) {
	q := url.Values{}
	q.Set("quiz_id", quizID)
	q.Set("user_id", userID)
	q.Set("name", name)
	conn, resp, err := websocket.DefaultDialer.Dial(wsBase+"/ws?"+q.Encode(), nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return nil, status, err
	}
	c := &wsClient{conn: conn, raw: make(chan wsEnvelope, 128)}
	go c.readLoop()
	return c, 101, nil
}

// tryDialWS attempts a join and, on the expected rejection, returns the HTTP
// status and error code the server wrote before declining the upgrade.
func tryDialWS(wsBase, quizID, userID, name string) (status int, code string) {
	q := url.Values{}
	q.Set("quiz_id", quizID)
	q.Set("user_id", userID)
	q.Set("name", name)
	conn, resp, err := websocket.DefaultDialer.Dial(wsBase+"/ws?"+q.Encode(), nil)
	if err == nil {
		_ = conn.Close()
		return 101, ""
	}
	if resp == nil {
		return 0, ""
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	var e errorResp
	_ = json.Unmarshal(body, &e)
	return resp.StatusCode, e.Code
}

func (c *wsClient) readLoop() {
	defer close(c.raw)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var env wsEnvelope
		if json.Unmarshal(data, &env) == nil {
			c.raw <- env
		}
	}
}

// waitFor returns the next envelope of the given type within timeout.
func (c *wsClient) waitFor(typ string, timeout time.Duration) (wsEnvelope, bool) {
	c.mu.Lock()
	for i, e := range c.pending {
		if e.Type == typ {
			c.pending = append(c.pending[:i], c.pending[i+1:]...)
			c.mu.Unlock()
			return e, true
		}
	}
	c.mu.Unlock()

	deadline := time.After(timeout)
	for {
		select {
		case e, ok := <-c.raw:
			if !ok {
				return wsEnvelope{}, false
			}
			if e.Type == typ {
				return e, true
			}
			c.mu.Lock()
			c.pending = append(c.pending, e)
			c.mu.Unlock()
		case <-deadline:
			return wsEnvelope{}, false
		}
	}
}

func (c *wsClient) send(questionID, answer string) error {
	b, _ := json.Marshal(wsOutbound{Type: msgSubmitAnswer, Payload: submitReq{QuestionID: questionID, Answer: answer}})
	return c.conn.WriteMessage(websocket.TextMessage, b)
}

func (c *wsClient) close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
