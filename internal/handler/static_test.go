package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gray/elsa-quiz/internal/store"
)

func TestServesIndexAtRoot(t *testing.T) {
	h := NewAPI(store.NewMemoryStore(), 10).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "Real-Time Quiz") {
		t.Errorf("body does not look like the app shell")
	}
}

// A made-up nested path must 404, not return the SPA shell — the root handler is
// a catch-all and must not swallow unknown paths.
func TestUnknownPathNotFound(t *testing.T) {
	h := NewAPI(store.NewMemoryStore(), 10).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope = %d, want 404", rec.Code)
	}
}

func TestRootDoesNotShadowAPI(t *testing.T) {
	h := NewAPI(store.NewMemoryStore(), 10).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status"`) {
		t.Fatalf("GET /api/health regressed: %d %s", rec.Code, rec.Body.String())
	}
}
