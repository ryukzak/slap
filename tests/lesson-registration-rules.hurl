# Lesson Registration Rules Test
# Tests: registration blocked with no submit record, blocked after lesson
#        starts, duplicate registration blocked, auto-revoke on new submit
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/lesson-registration-rules.hurl

# Setup: sign in as teacher — capture cookie from 303 response
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Rules Student
variable: task_id=task1
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

# Setup: create upcoming lesson — capture ID from the response
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 14:00
description: Registration rules test lesson

HTTP 200
[Captures]
upcoming_lesson_id: url regex "/lesson/([^/?#]+)"

# Setup: create past lesson — capture ID from the response
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2000-01-01
time: 10:00
description: Past lesson

HTTP 200
[Captures]
past_lesson_id: url regex "/lesson/([^/?#]+)"

# Setup: sign up student — capture cookie from 303 response
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

# Rule 1: Registration fails if student has no submit record
POST {{base_url}}/api/lesson/{{upcoming_lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 500
[Asserts]
body contains "no task entries found"

# Setup: student submits a record
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: First submission

HTTP 303

# Rule 2: Registration blocked for past lesson
POST {{base_url}}/api/lesson/{{past_lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 500
[Asserts]
body contains "registration is closed"

# Rule 3: Registration works for upcoming lesson
POST {{base_url}}/api/lesson/{{upcoming_lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Rule 4: Duplicate registration is blocked
POST {{base_url}}/api/lesson/{{upcoming_lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 500
[Asserts]
body contains "already registered"

# Rule 5: Auto-revoke — student submits new record, registration is revoked
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Updated submission after registering

HTTP 303

# Registration status should now show as revoked on task page
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Dropped"

# And dashboard shows Pending (the new record)
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Pending"

# Rule 6: Teacher extends deadline on past lesson — registration then succeeds
GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/api/lesson/{{past_lesson_id}}/extend
Cookie: user_data={{teacher_token}}
[FormParams]
date: 2099-12-31
time: 14:00

HTTP 303

POST {{base_url}}/api/lesson/{{past_lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200
