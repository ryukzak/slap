# Lesson Queue Flow Test
# Tests: teacher creates lesson → student registers task → teacher reviews
#        from lesson queue → task transitions to reviewed
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=student_$(date +%s) \
#   --variable teacher_id=123 \
#   tests/lesson-queue-flow.hurl

# Step 1: Sign in as teacher (already registered from earlier test)
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Queue Student
variable: task_id=task1
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

# Clear cookie jar before switching users
GET {{base_url}}/logout
HTTP 303

# Step 2: Teacher creates a lesson (far future)
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 14:00
description: Queue test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

# Step 3: Sign up student
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

# Clear cookie jar before switching users
GET {{base_url}}/logout
HTTP 303

# Step 4: Student submits a task record
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Task ready for lesson queue

HTTP 303

# Step 5: Student registers task to lesson
POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Step 6: Task page shows registered badge
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Queued"

# Step 7: Teacher views lesson queue and sees registered task
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Queue Student"
body contains "Task ready for lesson queue"
body contains "Queued"

# Step 8: Teacher submits review from lesson page context (using journal endpoint)
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: Lesson review feedback

HTTP 303

# Step 9: Lesson page now shows reviewed status
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Checked"

# Step 10: Student dashboard shows Feedback badge for the task
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Feedback"
