# Tag Detail Filters Test
# The tag detail page (/tags/{tag}):
#   1. No longer shows the tag itself as a per-row chip (it's redundant --
#      every row on this page already carries the tag being viewed).
#   2. Offers a score filter (Scored/No score pills) that can be toggled
#      independently via query params, hiding non-matching rows entirely
#      (not just visually) -- more useful than filtering by status, since
#      "does this have a score yet" is what a teacher actually cares about.
#   3. Has a [view task] button in the row's own top line (not a separate
#      row), linking to the task page where a teacher sees full information.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)34 \
#   tests/ui/tag-detail-filters.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_a_password=StudentAPass123!
variable: student_a_username=FilterAlice
variable: student_b_password=StudentBPass123!
variable: student_b_username=FilterBob
variable: student_b_id={{student_id}}1
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student A submits task1; teacher reviews with a leading score and
# the tag -- this row ends up scored.
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
content: Alice task1 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: 10 Great work \#filtertag

HTTP 303

# Step 3: Student B submits task2; teacher reviews with the tag but no
# leading number -- this row stays unscored.
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
content: Bob task2 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_b_id}}/task/task2/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Nice \#filtertag

HTTP 303

# Step 4: Default view (no filter params) shows both rows, no per-row tag
# chip, a [view task] link per row, Alice's score, and the two filter pills.
GET {{base_url}}/tags/filtertag
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@FilterAlice"
body contains "@FilterBob"
body contains "[view task]"
body contains "href=\"/user/{{student_id}}/task/task1\""
body contains "href=\"/user/{{student_b_id}}/task/task2\""
body not contains "href=\"/tags/filtertag\""
body contains "href=\"?scored=false&amp;unscored=true\""
body contains "text-green-300\">10<"

# Step 5: Turning the Scored pill off hides Alice's row and updates the count.
GET {{base_url}}/tags/filtertag?scored=false&unscored=true
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "@FilterAlice"
body contains "@FilterBob"
body contains "(1 of 2)"

# Step 6: Turning the No score pill off instead hides Bob's row.
GET {{base_url}}/tags/filtertag?scored=true&unscored=false
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@FilterAlice"
body not contains "@FilterBob"
body contains "(1 of 2)"

# Step 7: Turning both off leaves a filtered-empty message, distinct from
# "no tasks tagged at all".
GET {{base_url}}/tags/filtertag?scored=false&unscored=false
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "@FilterAlice"
body not contains "@FilterBob"
body contains "No tasks match the current score filter."

GET {{base_url}}/logout
HTTP 303
