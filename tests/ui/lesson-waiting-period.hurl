# Lesson registration waiting period Test
# Covers the per-task waiting period (configured in hours; task1 = 24h in the
# default config). After a teacher review, a student cannot immediately
# re-register the same task to a lesson:
#   1. The register list shows the "Waiting period: ..." message and a [not yet]
#      marker instead of a [register] button.
#   2. The register POST is rejected with a "waiting period" error.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"], task1 waiting_period_hours: 24).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/lesson-waiting-period.hurl

# Step 1: Sign in as teacher (signed up by earlier tests)
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Waiting Period Student
variable: task_id=task1
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
description: Lesson waiting period test

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

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

# Step 4: Student submits the task
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: First submission for waiting period test

HTTP 303

# Step 5: Student registers the task to the lesson (no prior review → allowed)
POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Step 6: Teacher reviews the student's task just now (starts the waiting period)
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: Reviewed — please address the feedback.

HTTP 303

# Step 7: Student resubmits so the latest record is a fresh submission
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Second submission right after the review

HTTP 303

# Step 8: Register list shows the waiting-period message and [not yet] marker
GET {{base_url}}/api/lessons?register=1&task_id={{task_id}}&student_id={{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Waiting period:"
body contains "[not yet]"
body not contains "/api/lesson/{{lesson_id}}/register"

# Step 9: Attempting to register anyway is rejected by the waiting period
POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 500
[Asserts]
body contains "waiting period"
