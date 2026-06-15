#!/usr/bin/env bash
# Orchestrate a black-box e2e run: build + boot the server, wait for /health,
# run the godog suite against it, then tear the server down.
#
#   E2E_TAGS   godog tag expression (default: blocking gate = "~@perf")
#   E2E_PORT   port to run the server on (default 8090, to avoid clashing with 8080)
#
# Exits with the godog status so callers (make e2e / CI) can gate on it.
set -euo pipefail

cd "$(dirname "$0")/.."

PORT="${E2E_PORT:-8090}"
TAGS="${E2E_TAGS:-~@perf}"
BASE="http://localhost:${PORT}"
WS="ws://localhost:${PORT}"

echo "==> building server"
go build -o bin/quiz ./cmd/server

echo "==> starting server on :${PORT}"
PORT="$PORT" BASE_POINTS="${BASE_POINTS:-10}" ./bin/quiz >e2e-server.log 2>&1 &
SERVER_PID=$!
cleanup() { kill "$SERVER_PID" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "==> waiting for /api/health"
ready=
for _ in $(seq 1 50); do
  if curl -fsS "${BASE}/api/health" >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.2
done
if [ -z "$ready" ]; then
  echo "!! server did not become healthy on ${BASE}" >&2
  exit 1
fi

echo "==> running godog (tags: ${TAGS})"
E2E_BASE_URL="$BASE" E2E_WS_URL="$WS" E2E_TAGS="$TAGS" \
  go test -tags e2e -count=1 ./tests/e2e
