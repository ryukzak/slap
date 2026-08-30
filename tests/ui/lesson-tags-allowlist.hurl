# Lesson Tags Allow-List Test
# The lesson page only shows tags listed in a task's own `tags` allow-list
# (conf/config.yaml has `task1.tags: ["approve"]`); any other tag stays
# hidden there even though it's still an active tag on the task itself.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"], task1.tags: ["approve"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)28 \
#   tests/ui/lesson-tags-allowlist.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=AllowlistStudent
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
time: 11:00
description: Tags allow-list test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 3: Student signs up, submits, and registers
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
content: My topic

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

# Step 4: Teacher reviews with an allow-listed tag and an unlisted one
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Looks good \#approve, double check with \#groupz

HTTP 303

# Step 5: Lesson page shows the allow-listed tag but not the unlisted one
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@AllowlistStudent"
body contains "href=\"/tags/approve\""
body not contains "href=\"/tags/groupz\""

# Step 6: The unlisted tag is still active on the task page itself
GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "href=\"/tags/groupz\""
body contains "href=\"/tags/approve\""

GET {{base_url}}/logout
HTTP 303
