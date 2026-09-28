# Student Page Task Timeline Test
# The "// your tasks" list on a student's profile now shows, per task: the
# first submission time, the first lesson registration time (if any), a
# count of teacher feedback records, and a time-scaled event timeline (a
# dot per record, positioned between that task's own first and last event --
# each task gets its own scale). The timeline must never leak a teacher's
# review content into the page (even via a hover tooltip) -- only a
# student's own "submit" records carry a content excerpt, matching the
# existing rule that a teacher's short remark never becomes a preview.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)41 \
#   tests/ui/user-task-timeline.hurl

# Step 1: Sign in as teacher, create a lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=TimelineStudent
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
description: Timeline test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits, registers into the lesson, teacher leaves a
# short review remark with a secret marker word that must never leak into
# the student's own task list -- not as visible text, not in a tooltip.
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: My real topic for the timeline test

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task2
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: SecretTeacherRemarkXYZ

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Student's own page shows the new per-task stats and timeline dots,
# but never the teacher's remark content
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "first submit:"
body contains "first queued:"
body contains "feedback: 1"
body contains "My real topic for the timeline test"
body not contains "SecretTeacherRemarkXYZ"
body contains "bg-yellow-400"
body contains "bg-green-400"

GET {{base_url}}/logout
HTTP 303
