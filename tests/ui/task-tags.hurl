# Task Tags Test
# Only a teacher can add or remove a tag by writing #tag / -#tag in a review;
# a student writing #tag in their own submission has no effect (it's just
# text). The task page shows every currently active tag (regardless of the
# config allow-list), and a later -#tag removes it.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)27 \
#   tests/ui/task-tags.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=TagsStudent
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Sign up a student
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

# Step 3: Student submits, trying to tag their own work — this has no effect
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_token}}
[FormParams]
content: My topic \#group3

HTTP 303

GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "href=\"/tags/group3\""

GET {{base_url}}/logout
HTTP 303

# Step 4: Teacher reviews and approves with a tag
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Nice work \#approve

HTTP 303

# Step 5: Task page shows the teacher's tag, linking to its browse-by-tag
# page, but still not the student's own attempted tag.
GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "href=\"/tags/approve\""
body not contains "href=\"/tags/group3\""

# Step 6: Teacher removes the tag
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: withdrawn -\#approve

HTTP 303

GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "href=\"/tags/approve\""

GET {{base_url}}/logout
HTTP 303
