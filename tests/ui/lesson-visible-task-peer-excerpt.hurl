# Lesson Visible-Task Peer Excerpt Test
# A task configured with `visible: true` (task5 in conf/config.yaml) lets any
# signed-in student see a content excerpt + tags for another student's record
# on a shared lesson page, instead of the usual "hidden" placeholder. A task
# without the flag (task1) keeps today's behavior: content stays hidden from
# peers entirely.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"], task5.visible: true).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)29 \
#   tests/ui/lesson-visible-task-peer-excerpt.hurl

# Step 1: Sign in as teacher, create a lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_a_password=StudentAPass123!
variable: student_a_username=VisibleAlice
variable: student_b_password=StudentBPass123!
variable: student_b_username=VisibleBob
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
time: 12:00
description: Visible task peer excerpt test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student A submits a visible task (task5) and a non-visible task
# (task1), then registers both to the lesson
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_a_password}}
username: {{student_a_username}}

HTTP 303
[Captures]
student_a_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task5/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Alice secret presentation topic

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_a_token}}
[FormParams]
taskRecordID: task5
studentID: {{student_id}}

HTTP 200

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Alice hidden lab1 content

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_a_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

# Step 2b: Teacher reviews the (already-registered) task5 submission and
# approves it with a tag — only a teacher can add a tag.
POST {{base_url}}/user/{{student_id}}/task/task5/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Approved topic \#approve

HTTP 303

# Step 3: Student B signs up and views the lesson page
POST {{base_url}}/signup
[FormParams]
id: {{student_b_id}}
password: {{student_b_password}}
username: {{student_b_username}}

HTTP 303
[Captures]
student_b_token: cookie "user_data"

GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_b_token}}

HTTP 200
[Asserts]
# Visible task: Student B sees Alice's excerpt and tag (as plain text, not
# a /tags/ link — that link is teacher-only).
body contains "Alice secret presentation topic"
body contains "#approve</span>"
# Non-visible task: content stays fully hidden from a peer.
body not contains "Alice hidden lab1 content"
body contains "Submission content is only visible to the author and teacher."

GET {{base_url}}/logout
HTTP 303

# Step 4: Teacher still sees full content for both, plus their own review
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Alice secret presentation topic"
body contains "Approved topic #approve"
body contains "Alice hidden lab1 content"
