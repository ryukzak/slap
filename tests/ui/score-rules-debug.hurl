# Score Rules Debug Endpoint
#
# Covers GET /user/{id}/score-debug — the trace that shows, per rule, the actual
# checked time vs the expected deadline, pass/fail per task, the counted number,
# and the rule result. Verifies both the plain-text and JSON renderings plus
# access control.
#
# Receives base_url, teacher_id, student_id from the runner.

# Student signs up.
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: teacher_password=TeacherPass123!
variable: student_username=ScoreDebug Student
variable: other_id=999000222
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# A second student (to check cross-student access is forbidden).
POST {{base_url}}/signup
[FormParams]
id: {{other_id}}
password: {{student_password}}
username: ScoreDebug Other

HTTP 303
[Captures]
other_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Teacher signs in.
POST {{base_url}}/signin
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# --- Plain-text trace (teacher viewing the student) ---
GET {{base_url}}/user/{{student_id}}/score-debug?format=text
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
header "Content-Type" contains "text/plain"
body contains "score-rule debug for @ScoreDebug Student"
body contains "Lab 1 hard deadline"
body contains "Mid-semester checkpoint"
body contains "condition: min_checked_before"
body contains "expected=before"
body contains "<never accepted>"
body contains "TOTAL applied effect:"

# --- JSON trace ---
GET {{base_url}}/user/{{student_id}}/score-debug
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.user_id" == "{{student_id}}"
jsonpath "$.username" == "ScoreDebug Student"
jsonpath "$.rules" count == 6
jsonpath "$.rules[0].rule" exists
jsonpath "$.rules[0].condition_kind" exists
jsonpath "$.rules[0].tasks" exists

# --- A student may inspect their own evaluation ---
GET {{base_url}}/user/{{student_id}}/score-debug?format=text
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "TOTAL applied effect:"

# --- A student may NOT inspect another student's evaluation ---
GET {{base_url}}/user/{{student_id}}/score-debug
Cookie: user_data={{other_token}}

HTTP 403

# --- Unauthenticated access is rejected ---
GET {{base_url}}/user/{{student_id}}/score-debug

HTTP 401
