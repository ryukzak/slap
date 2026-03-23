# Dashboard Role Sections Test
# Verifies that the teacher dashboard shows the lesson creation form
# and a created lesson, while the student dashboard does NOT show
# the creation form but does show lesson and task sections.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/dashboard-role-sections.hurl

# Step 1: Sign in as teacher (already registered from earlier tests)
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Dashboard Student
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

# Clear cookie jar before checking dashboard
GET {{base_url}}/logout
HTTP 303

# Step 2: Teacher dashboard shows lesson creation form
GET {{base_url}}/user/{{teacher_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "// add lesson"
body contains "id=\"lesson-description\""
body contains "name=\"date\""
body contains "name=\"time\""

# Step 3: Teacher creates a lesson
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 14:00
description: Dashboard test lesson

HTTP 200

# Step 4: Teacher dashboard shows the created lesson
GET {{base_url}}/user/{{teacher_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Dashboard test lesson"

# Clear cookie jar before switching to student
GET {{base_url}}/logout
HTTP 303

# Step 5: Sign up student
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

# Step 6: Student dashboard does NOT show lesson creation form
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "// lessons"
body contains "// your tasks"
body not contains "// add lesson"
body not contains "id=\"lesson-description\""
