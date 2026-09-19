# Teachers Score Log Test
# The teachers page (/teachers) shows a compact, teacher-only score log at
# the bottom of the page: the most recent teacher reviews that set a numeric
# score, newest first — time (with seconds), teacher (linked), student
# (linked), task (linked), score. This test scores two tasks (one 7, one 0)
# and asserts both appear in the log, in newest-first order, with the 0
# score shown rather than hidden, the teacher name linked to their profile,
# and the 10/100/500 size toggle.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)38 \
#   tests/ui/teachers-score-log.hurl

# Step 1: Sign in as teacher, sign up student
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=ScoreLogPass123!
variable: student_username=ScoreLog Student
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

# Step 2: Student submits two tasks
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Score log task1 submission

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Score log task2 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Teacher reviews task1 with score 7, then task2 with score 0
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: 7 solid work

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: 0 needs a full rework

HTTP 303

# Step 4: /users no longer carries a score log at all
GET {{base_url}}/users
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "score log"

# Step 5: The teachers page shows both entries in the score log — task2/0
# (newer) before task1/7 (older) — each row linking to its task and to the
# reviewing teacher's profile, with seconds in the timestamp, plus the size
# toggle offering 10/100/500.
GET {{base_url}}/teachers
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "score log"
body contains "href=\"/user/{{teacher_id}}\">@Test Teacher</a>"
body matches "(?s)/user/{{student_id}}/task/task2\">Lab 2</a>\\s*</td>\\s*<td class=\"py-1 pl-2 text-right text-green-300\">\\s*0\\s*</td>"
body matches "(?s)/user/{{student_id}}/task/task1\">Lab 1</a>\\s*</td>\\s*<td class=\"py-1 pl-2 text-right text-green-300\">\\s*7\\s*</td>"
body matches "(?s)/user/{{student_id}}/task/task2.*?/user/{{student_id}}/task/task1"
body matches "\\d{2}:\\d{2}:\\d{2}\\(\\w+\\)/\\d{2}:\\d{2}:\\d{2}\\(\\w+\\)"
body contains "logLimit=100"
body contains "logLimit=500"

# Step 6: Students cannot see the teachers page at all (teacher-only)
GET {{base_url}}/logout
HTTP 303

GET {{base_url}}/teachers
Cookie: user_data={{student_token}}

HTTP 403
