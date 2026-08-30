# Tags Browse Page Test
# /tags lists every currently active tag with a count; /tags/{tag} lists every
# student+task pair carrying it. Both are teacher-only.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)30 \
#   tests/ui/tags-browse-page.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=BrowseStudent
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student signs up and submits; only the teacher's review can tag it
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
content: Browsing test

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Reviewed \#browsetag

HTTP 303

# Step 3: Teacher browses the tag index
GET {{base_url}}/tags
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "href=\"/tags/browsetag\""

# Step 4: Teacher browses the tag detail page
GET {{base_url}}/tags/browsetag
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@BrowseStudent"
body contains "href=\"/user/{{student_id}}/task/task1\""

# Step 5: A student cannot browse either page
GET {{base_url}}/tags
Cookie: user_data={{student_token}}

HTTP 403

GET {{base_url}}/tags/browsetag
Cookie: user_data={{student_token}}

HTTP 403
