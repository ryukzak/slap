# Lesson Teacher Summary Test
# The lesson page now shows a teacher-only "lesson summary" panel at the top
# aggregating how the lesson finished: how many task records ended up
# checked/queued/dropped, and the score distribution of the checked ones —
# explicitly including a score of 0, which the per-user statistics page
# excludes. This test checks one task with a 0 score and leaves a second
# queued, then asserts the summary reflects both, and that students never
# see the panel at all.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)37 \
#   tests/ui/lesson-teacher-summary.hurl

# Step 1: Sign in as teacher, create a lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_a_password=SummaryAPass123!
variable: student_a_username=SummaryChecked
variable: student_b_password=SummaryBPass123!
variable: student_b_username=SummaryQueued
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
time: 18:00
description: Teacher summary test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

# Step 2: Before any registrations, the summary panel is not shown
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "lesson summary"
body not contains "checked:"

GET {{base_url}}/logout
HTTP 303

# Step 3: Student A submits+registers task1, and the teacher scores it 0
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
content: Summary checked submission

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
content: 0 needs a full rework

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 4: Student B submits+registers task2, and stays queued (unreviewed)
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
content: Summary queued submission

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_b_token}}
[FormParams]
taskRecordID: task2
studentID: {{student_b_id}}

HTTP 200

# Step 5: Student never sees the teacher-only summary panel
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_b_token}}

HTTP 200
[Asserts]
body not contains "lesson summary"

GET {{base_url}}/logout
HTTP 303

# Step 6: Teacher's summary panel counts one checked, one queued, and shows
# the 0 score rather than hiding it.
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "checked: 1"
body contains "queued: 1"
body contains "min:0"
body contains "0:&nbsp;1"

GET {{base_url}}/logout
HTTP 303
