#!/usr/bin/env bash
# Run all Playwright e2e tests against a fresh server instance.
# Builds the binary, starts the server with a temp DB, runs all tests,
# then stops the server. Exits with the test status code.
#
# Usage: ./tests/run-e2e.sh [port]
#   port - port to run the test server on (default: 18090)
#
# Example: ./tests/run-e2e.sh 19090

set -euo pipefail

PORT=${1:-18090}
BINARY=./slap
TEST_DB=tmp/e2e-test.db
BASE_URL="http://localhost:${PORT}"

# Build
echo "Building..."
go build -o "$BINARY" .

# Prepare temp DB directory
mkdir -p tmp
rm -f "$TEST_DB"

# Kill any process already on the port
lsof -ti :"$PORT" | xargs kill -9 2>/dev/null || true

# Start server with isolated test DB
SLAP_DB="$TEST_DB" SLAP_JWT_SECRET="test-secret-for-ci-only" "$BINARY" -port "$PORT" &
SERVER_PID=$!
echo "Server started (PID $SERVER_PID) on port $PORT"

# Wait for server to be ready
echo "Waiting for server..."
for i in $(seq 1 20); do
    curl -sf "$BASE_URL/" > /dev/null 2>&1 && break
    sleep 0.5
done

STATUS=0
(cd e2e && E2E_BASE_URL="$BASE_URL" npx playwright test) || STATUS=$?

# Stop server
kill "$SERVER_PID" 2>/dev/null
rm -f "$TEST_DB"

if [ "$STATUS" -eq 0 ]; then
    echo "All e2e tests passed."
else
    echo "Some e2e tests failed."
fi

exit "$STATUS"
