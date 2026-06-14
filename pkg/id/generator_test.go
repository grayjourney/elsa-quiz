package id

import "testing"

func TestNew_ReturnsNonEmptyString(t *testing.T) {
	if got := New(); got == "" {
		t.Fatalf("New() = %q, want non-empty string", got)
	}
}

func TestNew_ReturnsUniqueIDs(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := range n {
		id := New()
		if _, dup := seen[id]; dup {
			t.Fatalf("New() produced a duplicate ID %q after %d generations", id, i)
		}
		seen[id] = struct{}{}
	}
}
