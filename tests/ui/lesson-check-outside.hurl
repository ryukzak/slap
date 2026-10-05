# Outside-Lesson Check Test
# A teacher checking a queued task from the task page, far from the lesson's
# start, is checking outside that lesson. The registration is dropped first —
# by the teacher, not the student — so the lesson neither keeps the student in
# its queue nor claims the check: the summary reports it as "checked outside",
# separately from both "checked" and "dropped".
#
# A review that declares a lesson_id (the lesson page's own review form) is
# that lesson's work regardless of the clock; tests/ui/lesson-teacher-summary
# covers that side.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)44 \
#   tests/ui/lesson-check-outside.hurl

# Step 1: Teacher creates a lesson far in the future, well outside the window
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=OutsidePass123!
variable: student_username=OutsideCheck
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
time: 18:00
description: Outside check test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits and registers task1
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
content: Outside check submission

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

# Step 3: The teacher's task page warns that a check here leaves the lesson
GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "drops the task from the queue first"
body not contains "counts as this lesson's check"

# Step 4: Teacher checks from the task page, declaring no lesson
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: 7 checked between lessons

HTTP 303

# Step 5: The lesson claims neither the queue slot nor the check, and does not
# report a drop the student never made.
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "checked: 0"
body contains "queued: 0"
body contains "dropped: 0"
body contains "checked outside: 1"
body not contains "min:7"

# Step 6: The row is held back with the revoked ones, and shows as a check
# taken outside rather than a drop.
GET {{base_url}}/lesson/{{lesson_id}}?showRevoked=true&sort=submit-ord
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Checked outside"
body not contains "Dropped"

# Step 7: The student's journal records the teacher as the one who removed the
# registration, not the student withdrawing.
GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Removed"
body not contains "Dropped"
body contains "@Test Teacher"
body contains "Checked"

GET {{base_url}}/logout
HTTP 303
