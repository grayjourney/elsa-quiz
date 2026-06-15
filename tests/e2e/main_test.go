//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cucumber/godog"
)

// TestMain waits for the target server to report healthy before the suite runs,
// so a freshly-started server (make e2e) never races the first scenario.
func TestMain(m *testing.M) {
	waitForServer(envOr("E2E_BASE_URL", "http://localhost:8080"), 15*time.Second)
	os.Exit(m.Run())
}

func waitForServer(base string, within time.Duration) {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/api/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   envOr("GODOG_FORMAT", "pretty"),
			Paths:    []string{"../../features"},
			Tags:     os.Getenv("E2E_TAGS"),
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("e2e: non-zero godog status (see failures above)")
	}
}

// InitializeScenario is invoked once per scenario; each gets a fresh World.
func InitializeScenario(ctx *godog.ScenarioContext) {
	w := newWorld()
	ctx.After(func(c context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		w.close()
		return c, nil
	})
	registerOpsSteps(ctx, w)
	registerSessionSteps(ctx, w)
	registerLifecycleSteps(ctx, w)
	registerScoreSteps(ctx, w)
	registerParticipationSteps(ctx, w)
	registerLeaderboardSteps(ctx, w)
	registerReliabilitySteps(ctx, w)
	registerValidationSteps(ctx, w)
	registerPerfSteps(ctx, w)
}
