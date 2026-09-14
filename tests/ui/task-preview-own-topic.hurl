# Task Preview Own Topic Test
# The "// your tasks" list on a student's profile previews each task with
# the student's own latest message (their real topic), never a teacher's
# newer, short tag-only review remark such as "Reviewed #approve".
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)31 \
#   tests/ui/task-preview-own-topic.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=PreviewStudent
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits their real topic
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
content: Implemented stack and queue with tests

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Teacher leaves a short, tag-only approval remark — this becomes
# the newest record, but must not become the preview text.
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Reviewed \#approve

HTTP 303

# Step 4: Student's own dashboard still previews their own topic message
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Implemented stack and queue with tests"
body not contains "Reviewed"
