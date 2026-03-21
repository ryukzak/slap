# Task Page Registered Lesson Info Test
# Verifies the task page shows full lesson details (teacher name,
# description, review counts) when a task is registered to a lesson.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/task-registered-lesson-info.hurl

# Step 1: Sign in as teacher (already registered from earlier tests)
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Lesson Info Student
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

# Step 2: Teacher creates a lesson
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 14:00
description: Lesson info test

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

GET {{base_url}}/logout
HTTP 303

# Step 4: Student submits a task record
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Submission for lesson info test

HTTP 303

# Step 5: Student registers task to lesson
POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Step 6: Task page shows full registered lesson info
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Queued"
body contains "@Test Teacher"
body contains "Lesson info test"
body contains "[0/1]"
