package handler

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeConn struct {
	mu     sync.Mutex
	closed bool
	recv   chan []byte
}

func newFakeConn() *fakeConn { return &fakeConn{recv: make(chan []byte, 16)} }

func (f *fakeConn) WriteMessage(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errors.New("closed")
	}
	f.recv <- data
	return nil
}

func (f *fakeConn) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

func waitMsg(t *testing.T, f *fakeConn) []byte {
	t.Helper()
	select {
	case m := <-f.recv:
		return m
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
		return nil
	}
}

func TestConnectionManager_RegisterAndCount(t *testing.T) {
	m := NewConnectionManager()
	m.Register("QUIZ-1", "alice", newFakeConn())
	m.Register("QUIZ-1", "bob", newFakeConn())
	if got := m.Count("QUIZ-1"); got != 2 {
		t.Errorf("Count = %d, want 2", got)
	}
}

func TestConnectionManager_Broadcast_ReachesAll(t *testing.T) {
	m := NewConnectionManager()
	a, b := newFakeConn(), newFakeConn()
	m.Register("QUIZ-1", "alice", a)
	m.Register("QUIZ-1", "bob", b)

	m.Broadcast("QUIZ-1", []byte("hello"))

	if got := string(waitMsg(t, a)); got != "hello" {
		t.Errorf("alice got %q, want hello", got)
	}
	if got := string(waitMsg(t, b)); got != "hello" {
		t.Errorf("bob got %q, want hello", got)
	}
}

func TestConnectionManager_Unregister_StopsDelivery(t *testing.T) {
	m := NewConnectionManager()
	a := newFakeConn()
	m.Register("QUIZ-1", "alice", a)
	m.Unregister("QUIZ-1", "alice")
	if got := m.Count("QUIZ-1"); got != 0 {
		t.Errorf("Count after unregister = %d, want 0", got)
	}
	m.Broadcast("QUIZ-1", []byte("hello"))
	select {
	case <-a.recv:
		t.Error("unregistered client received a broadcast")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestConnectionManager_SendTo_OnlyTarget(t *testing.T) {
	m := NewConnectionManager()
	a, b := newFakeConn(), newFakeConn()
	m.Register("QUIZ-1", "alice", a)
	m.Register("QUIZ-1", "bob", b)

	m.SendTo("QUIZ-1", "alice", []byte("just-alice"))
	if got := string(waitMsg(t, a)); got != "just-alice" {
		t.Errorf("alice got %q", got)
	}
	select {
	case <-b.recv:
		t.Error("bob received a message meant only for alice")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestConnectionManager_ConcurrentAccess_NoRace(t *testing.T) {
	m := NewConnectionManager()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u := string(rune('a' + i%26))
			m.Register("QUIZ-1", u, newFakeConn())
			m.Broadcast("QUIZ-1", []byte("x"))
			m.Count("QUIZ-1")
			m.Unregister("QUIZ-1", u)
		}(i)
	}
	wg.Wait()
}
