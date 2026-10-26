# Lesson date/time edit Test
# The lesson's teacher can move the lesson to another date/time:
#   1. The lesson page shows an "edit date" form for the owning teacher,
#      pre-filled with the current lesson date and time.
#   2. POST /api/lesson/{id}/datetime moves the lesson.
#   3. Missing or malformed values are rejected; non-teachers cannot change it.
#
# Prerequisites: server running with default config (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/lesson-datetime-edit.hurl

# Step 1: Sign in as teacher (signed up by earlier tests)
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Datetime Edit Student
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

# Step 2: Teacher creates a lesson
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 10:00
description: Lesson datetime edit test

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

# Step 3: Lesson page shows the edit form pre-filled with the current date
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "/api/lesson/{{lesson_id}}/datetime"
body contains "[set date]"
body contains "value=\"2099-12-31\""
body contains "value=\"10:00\""
body contains "31.12.2099"

# Step 4: Teacher moves the lesson
POST {{base_url}}/api/lesson/{{lesson_id}}/datetime
Cookie: user_data={{teacher_token}}
[FormParams]
date: 2099-11-15
time: 14:30

HTTP 303

GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "15.11.2099"
body contains "14:30"
body contains "value=\"2099-11-15\""
body contains "value=\"14:30\""
body not contains "31.12.2099"

# Step 5: Missing or malformed values are rejected
POST {{base_url}}/api/lesson/{{lesson_id}}/datetime
Cookie: user_data={{teacher_token}}
[FormParams]
date: 2099-11-15

HTTP 400

POST {{base_url}}/api/lesson/{{lesson_id}}/datetime
Cookie: user_data={{teacher_token}}
[FormParams]
date: not-a-date
time: 14:30

HTTP 400

GET {{base_url}}/logout
HTTP 303

# Step 6: Sign up a student — students cannot move the lesson
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

POST {{base_url}}/api/lesson/{{lesson_id}}/datetime
Cookie: user_data={{student_token}}
[FormParams]
date: 2099-01-01
time: 09:00

HTTP 403

# Step 7: The student does not see the edit form, and the date is unchanged
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "/api/lesson/{{lesson_id}}/datetime"
body contains "15.11.2099"
