# Lesson Review Timestamp Test
# The lesson page's review panel showed only the reviewer's name, with no
# indication of when the review was placed. This test registers a task to a
# lesson, has the teacher review it, and asserts the lesson page's review
# block now includes a timestamp next to the reviewer's name.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)35 \
#   tests/ui/lesson-review-timestamp.hurl

# Step 1: Sign in as teacher, create a lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Timestamp Student
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 15:00
description: Review timestamp test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits and registers a task to the lesson
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Timestamp test submission

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

# Step 3: Teacher reviews the task from the lesson queue
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: 9 nice work

HTTP 303

# Step 4: Lesson page's review block shows the reviewer's name followed by a
# timestamp separator, not just the bare name.
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "review by @Test Teacher ·"

GET {{base_url}}/logout
HTTP 303
