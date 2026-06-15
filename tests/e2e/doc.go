// Package e2e holds the black-box end-to-end test suite: it drives a running
// quiz server over real HTTP + WebSocket and asserts only observable behavior.
//
// The suite is gated behind the `e2e` build tag so the fast unit/integration
// pass (`go test ./...`) never boots a server. Run it with `make e2e`
// (orchestrates the server) or directly against a running target:
//
//	E2E_BASE_URL=http://localhost:8080 E2E_WS_URL=ws://localhost:8080 \
//	  go test -tags e2e ./tests/e2e
//
// See docs/05-e2e-test-plan.md for the design and the tier/gate policy.
package e2e
