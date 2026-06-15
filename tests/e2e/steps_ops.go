//go:build e2e

package e2e

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func registerOpsSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^the quiz server is running$`, func() error { return nil })
	ctx.Step(`^(\d+) quiz sessions are active$`, w.sessionsAreActive)
	ctx.Step(`^a quiz session has processed at least one answer$`, w.processedOneAnswer)
	ctx.Step(`^a client requests "GET ([^"]+)"$`, w.clientGets)
	ctx.Step(`^the response status is (\d+)$`, w.responseStatusIs)
	ctx.Step(`^the body reports status "([^"]+)"$`, w.bodyReportsStatus)
	ctx.Step(`^the body reports an active session count of at least (\d+)$`, w.activeCountAtLeast)
	ctx.Step(`^the body is in Prometheus exposition format$`, w.bodyIsPrometheus)
	ctx.Step(`^it includes "([^"]+)", "([^"]+)", and "([^"]+)"$`, w.bodyIncludesThree)
}

func (w *World) sessionsAreActive(n int) error {
	for range n {
		if err := w.createSession("manual", 0, 1); err != nil {
			return err
		}
		if err := w.startREST(); err != nil {
			return err
		}
	}
	return nil
}

func (w *World) processedOneAnswer() error {
	if err := w.createSession("manual", 0, 1); err != nil {
		return err
	}
	if err := w.joinREST("Alice"); err != nil {
		return err
	}
	if err := w.startREST(); err != nil {
		return err
	}
	if err := w.submitREST("Alice", "Q1", "went"); err != nil {
		return err
	}
	if w.lastStatus != 200 {
		return fmt.Errorf("seed answer status = %d, want 200 (code=%s)", w.lastStatus, w.lastErr.Code)
	}
	return nil
}

func (w *World) clientGets(path string) error { return w.req("GET", path, "", nil) }

func (w *World) responseStatusIs(code int) error {
	if w.lastStatus != code {
		return fmt.Errorf("status = %d, want %d (body: %s)", w.lastStatus, code, string(w.lastRaw))
	}
	return nil
}

func (w *World) bodyReportsStatus(want string) error {
	var h healthResp
	if err := w.decodeLast(&h); err != nil {
		return err
	}
	if h.Status != want {
		return fmt.Errorf("health status = %q, want %q", h.Status, want)
	}
	return nil
}

func (w *World) activeCountAtLeast(n int) error {
	var h healthResp
	if err := w.decodeLast(&h); err != nil {
		return err
	}
	if h.ActiveSessions < n {
		return fmt.Errorf("activeSessions = %d, want >= %d", h.ActiveSessions, n)
	}
	return nil
}

func (w *World) bodyIsPrometheus() error {
	s := string(w.lastRaw)
	if !strings.Contains(s, "# HELP") && !strings.Contains(s, "# TYPE") {
		return fmt.Errorf("body is not Prometheus exposition format: %.80q", s)
	}
	return nil
}

func (w *World) bodyIncludesThree(a, b, c string) error {
	s := string(w.lastRaw)
	for _, name := range []string{a, b, c} {
		if !strings.Contains(s, name) {
			return fmt.Errorf("metrics output missing %q", name)
		}
	}
	return nil
}
