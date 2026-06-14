package handler

import "sync"

// Conn is the minimal transport the manager needs, decoupling it from gorilla
// so it is unit-testable with a fake. Writes are serialized by one goroutine
// per client (a WS connection must not be written concurrently).
type Conn interface {
	WriteMessage(data []byte) error
	Close() error
}

const sendBuffer = 16

type client struct {
	userID string
	conn   Conn
	send   chan []byte
}

func (c *client) writePump() {
	for msg := range c.send {
		if err := c.conn.WriteMessage(msg); err != nil {
			break
		}
	}
	_ = c.conn.Close()
}

type ConnectionManager struct {
	mu       sync.RWMutex
	sessions map[string]map[string]*client
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{sessions: make(map[string]map[string]*client)}
}

func (m *ConnectionManager) Register(quizID, userID string, conn Conn) {
	c := &client{userID: userID, conn: conn, send: make(chan []byte, sendBuffer)}
	m.mu.Lock()
	if m.sessions[quizID] == nil {
		m.sessions[quizID] = make(map[string]*client)
	}
	if old, ok := m.sessions[quizID][userID]; ok {
		close(old.send)
	}
	m.sessions[quizID][userID] = c
	m.mu.Unlock()
	go c.writePump()
}

func (m *ConnectionManager) Unregister(quizID, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conns, ok := m.sessions[quizID]
	if !ok {
		return
	}
	if c, ok := conns[userID]; ok {
		close(c.send)
		delete(conns, userID)
	}
	if len(conns) == 0 {
		delete(m.sessions, quizID)
	}
}

func (m *ConnectionManager) Broadcast(quizID string, msg []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.sessions[quizID] {
		trySend(c, msg)
	}
}

func (m *ConnectionManager) BroadcastExcept(quizID, exceptUserID string, msg []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for uid, c := range m.sessions[quizID] {
		if uid == exceptUserID {
			continue
		}
		trySend(c, msg)
	}
}

func (m *ConnectionManager) SendTo(quizID, userID string, msg []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.sessions[quizID][userID]; ok {
		trySend(c, msg)
	}
}

func (m *ConnectionManager) Count(quizID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions[quizID])
}

func (m *ConnectionManager) UserIDs(quizID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.sessions[quizID]))
	for uid := range m.sessions[quizID] {
		ids = append(ids, uid)
	}
	return ids
}

// trySend never blocks: a client whose buffer is full is treated as too slow and
// the message is dropped rather than stalling the broadcast to everyone else.
func trySend(c *client, msg []byte) {
	select {
	case c.send <- msg:
	default:
	}
}
