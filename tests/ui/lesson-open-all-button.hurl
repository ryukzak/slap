# Lesson Open-All Button Test
# The lesson page's "open queued" button only unrolled task panels with
# data-status="register" (still-queued work). It now unrolls every task
# panel on the page — queued, checked, and dropped alike — and is relabeled
# "[open all]". This test creates a lesson with a checked task, a dropped
# task, and a still-queued task, then asserts the button's label and its
# data-target selector no longer filter by status.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)36 \
#   tests/ui/lesson-open-all-button.hurl

# Step 1: Sign in as teacher, create a lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_a_password=OpenAllAPass123!
variable: student_a_username=OpenAllChecked
variable: student_b_password=OpenAllBPass123!
variable: student_b_username=OpenAllDropped
variable: student_b_id={{student_id}}1
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
time: 16:00
description: Open-all button test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student A submits+registers task1, and the teacher checks it
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_a_password}}
username: {{student_a_username}}

HTTP 303
[Captures]
student_a_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Open-all checked submission

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_a_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: 7 checked

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Student B submits+registers task2, then drops the registration
POST {{base_url}}/signup
[FormParams]
id: {{student_b_id}}
password: {{student_b_password}}
username: {{student_b_username}}

HTTP 303
[Captures]
student_b_token: cookie "user_data"

POST {{base_url}}/user/{{student_b_id}}/task/task2/journal
Cookie: user_data={{student_b_token}}
[FormParams]
content: Open-all dropped submission

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_b_token}}
[FormParams]
taskRecordID: task2
studentID: {{student_b_id}}

HTTP 200

POST {{base_url}}/api/lesson/{{lesson_id}}/unregister
Cookie: user_data={{student_b_token}}
[FormParams]
taskID: task2

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 4: Teacher views the lesson with the dropped record shown too, and
# the button reads "[open all]" with a target selector that no longer
# filters details by data-status.
GET {{base_url}}/lesson/{{lesson_id}}?showRevoked=true
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "[open all]"
body not contains "[open queued]"
body contains "data-target=\"#lesson-task-records > ul > li > details\">"

GET {{base_url}}/logout
HTTP 303
