#!/usr/bin/env bash
# Run all Hurl integration tests against a fresh server instance.
# Builds the binary, starts the server with a temp DB, runs all test files,
# then stops the server. Exits with the test status code.
#
# Usage: ./tests/run-hurl.sh [port] [teacher_id]
#   port       - port to run the test server on (default: 18080)
#   teacher_id - teacher ID matching the server config (default: 123)
#
# Example: ./tests/run-hurl.sh 19090 456

set -euo pipefail

PORT=${1:-18080}
TEACHER_ID=${2:-123}
BINARY=./slap
TEST_DB=tmp/test-slap.db
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
for _ in $(seq 1 20); do
    curl -sf "$BASE_URL/" > /dev/null 2>&1 && break
    sleep 0.5
done

HURL_VARS=(
    --variable "base_url=$BASE_URL"
    --variable "teacher_id=$TEACHER_ID"
)

TIMESTAMP=$(date +%s)
STATUS=0

run_test() {
    local file=$1
    shift
    hurl --test "${HURL_VARS[@]}" "$@" "$file" || STATUS=$?
}

run_test tests/task-submit-flow.hurl \
    --variable "student_id=${TIMESTAMP}0"

run_test tests/teacher-review-flow.hurl \
    --variable "student_id=${TIMESTAMP}1"

run_test tests/lesson-queue-flow.hurl \
    --variable "student_id=${TIMESTAMP}2"

run_test tests/lesson-registration-rules.hurl \
    --variable "student_id=${TIMESTAMP}3"

run_test tests/access-control.hurl \
    --variable "student_a_id=${TIMESTAMP}4" \
    --variable "student_b_id=${TIMESTAMP}5"

run_test tests/settings-flow.hurl \
    --variable "student_id=${TIMESTAMP}6" \
    --variable "other_student_id=${TIMESTAMP}9"

run_test tests/password-reset-flow.hurl \
    --variable "student_id=${TIMESTAMP}7" \
    --variable "teacher_id=$TEACHER_ID"

run_test tests/lesson-cascade-delete.hurl \
    --variable "student_id=${TIMESTAMP}8"

run_test tests/reset-request-flow.hurl \
    --variable "student_id=${TIMESTAMP}14" \
    --variable "teacher_id=$TEACHER_ID"

run_test tests/score-rules.hurl \
    --variable "student_id=${TIMESTAMP}16"

run_test tests/ui/user-list-student-row.hurl \
    --variable "student_id=${TIMESTAMP}10"

run_test tests/ui/task-registered-lesson-info.hurl \
    --variable "student_id=${TIMESTAMP}11"

run_test tests/ui/dashboard-role-sections.hurl \
    --variable "student_id=${TIMESTAMP}12"

run_test tests/ui/task-markdown-rendering.hurl \
    --variable "student_id=${TIMESTAMP}13"

run_test tests/ui/lesson-student-queue-visibility.hurl \
    --variable "student_id=${TIMESTAMP}15"

run_test tests/ui/score-rules-student-visibility.hurl \
    --variable "student_id=${TIMESTAMP}17" \
    --variable "teacher_id=${TEACHER_ID}"

run_test tests/ui/users-total-effect-column.hurl \
    --variable "student_id=${TIMESTAMP}18" \
    --variable "teacher_id=${TEACHER_ID}"

# Stop server
kill "$SERVER_PID" 2>/dev/null
rm -f "$TEST_DB"

if [ "$STATUS" -eq 0 ]; then
    echo "All Hurl tests passed."
else
    echo "Some Hurl tests failed."
fi

exit "$STATUS"
