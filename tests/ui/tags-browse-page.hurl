# Tags Browse Page Test
# /tags lists every currently active tag with a count; /tags/{tag} lists
# every student+task pair carrying it. Both pages are open to any signed-in
# user: a student always sees their own rows, and otherwise only rows for a
# peer-visible task (task5 has `visible: true`; task1 does not). The excerpt
# shown for a row is the student's own latest message, never a teacher's
# short tag-only review remark.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"], task1.visible unset, task5.visible: true).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)30 \
#   tests/ui/tags-browse-page.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_a_password=StudentAPass123!
variable: student_a_username=BrowseAlice
variable: student_c_password=StudentCPass123!
variable: student_c_username=BrowseCarol
variable: student_c_id={{student_id}}2
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student A submits on a non-visible task (task1) and a peer-visible
# task (task5), each with a real topic message
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
content: Browsing test topic

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task5/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Peer visible topic

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Teacher reviews both with a short, tag-only remark — this is the
# comment that must NOT surface as the excerpt.
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Reviewed \#browsetag

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task5/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Nice \#peertag

HTTP 303

# Step 4: Teacher browses the tag index — sees both tags
GET {{base_url}}/tags
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "href=\"/tags/browsetag\""
body contains "href=\"/tags/peertag\""

# Step 5: Teacher browses the browsetag detail page — excerpt is Alice's own
# topic message, not the teacher's "Reviewed" remark.
GET {{base_url}}/tags/browsetag
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@BrowseAlice"
body contains "href=\"/user/{{student_id}}/task/task1\""
body contains "Browsing test topic"
body not contains "Reviewed"

# Step 6: Student A (the row's own student) can browse both pages and sees
# her own row with the same correct excerpt.
GET {{base_url}}/tags
Cookie: user_data={{student_a_token}}

HTTP 200
[Asserts]
body contains "href=\"/tags/browsetag\""

GET {{base_url}}/tags/browsetag
Cookie: user_data={{student_a_token}}

HTTP 200
[Asserts]
body contains "@BrowseAlice"
body contains "Browsing test topic"

# Step 7: An unrelated student (Carol) can browse the pages (no more 403),
# but only sees rows for the peer-visible task (task5/peertag) — the
# non-visible task1/browsetag row is filtered out entirely.
POST {{base_url}}/signup
[FormParams]
id: {{student_c_id}}
password: {{student_c_password}}
username: {{student_c_username}}

HTTP 303
[Captures]
student_c_token: cookie "user_data"

GET {{base_url}}/tags
Cookie: user_data={{student_c_token}}

HTTP 200
[Asserts]
body contains "href=\"/tags/peertag\""
body not contains "href=\"/tags/browsetag\""

GET {{base_url}}/tags/peertag
Cookie: user_data={{student_c_token}}

HTTP 200
[Asserts]
body contains "@BrowseAlice"
body contains "Peer visible topic"

GET {{base_url}}/tags/browsetag
Cookie: user_data={{student_c_token}}

HTTP 200
[Asserts]
body not contains "@BrowseAlice"
body contains "No tasks currently tagged"

GET {{base_url}}/logout
HTTP 303
