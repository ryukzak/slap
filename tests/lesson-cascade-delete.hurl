# Lesson Cascade Delete Test
# Tests: teacher creates lesson → student registers → teacher deletes lesson →
#        task record reverts to submit state; lesson returns 404
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=student_$(date +%s) \
#   --variable teacher_id=123 \
#   tests/lesson-cascade-delete.hurl

# Step 1: Sign in as teacher (registered by teacher-review-flow.hurl)
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: task_id=task1
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Teacher creates a future lesson
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-11-30
time: 10:00
description: Cascade delete test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

# Step 3: Sign up student
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: Cascade Student

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 4: Student submits a task
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Ready for cascade delete test

HTTP 303

# Step 5: Student registers task to lesson
POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Step 6: Task shows as Queued
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Queued"

# Step 7: Teacher deletes the lesson
DELETE {{base_url}}/api/lessons/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200

# Step 8: Lesson page now returns 404
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_token}}

HTTP 404

# Step 9: Task record reverted to submit — no longer shows Queued
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "Queued"

# Step 10: Student can register to a different lesson (task is back to submit)
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-15
time: 11:00
description: Second lesson after cascade delete

HTTP 200
[Captures]
lesson2_id: url regex "/lesson/([^/?#]+)"

POST {{base_url}}/api/lesson/{{lesson2_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200
