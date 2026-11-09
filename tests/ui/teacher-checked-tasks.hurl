# Teacher Checked Tasks Test
# A teacher's profile page shows a teacher-only "checked tasks" table: the
# reviews that teacher left, newest first — time (linked to the review),
# student (linked), task (linked), score ("—" when the review has none) —
# filterable by task with one button per task. A student's own page has no
# such table, and a student cannot open the teacher's page at all.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)51 \
#   tests/ui/teacher-checked-tasks.hurl

# Step 1: Sign in as teacher, sign up student
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=CheckedPass123!
variable: student_username=Checked Student
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
content: Checked tasks task1 submission

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Checked tasks task2 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Teacher scores task1 and leaves a remark without a score on task2
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: 7 solid work

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: please add more detail

HTTP 303

# Step 4: Both reviews are listed on the teacher's page, newest first
GET {{base_url}}/user/{{teacher_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "// checked tasks ("
body contains "href=\"/user/{{student_id}}\">@{{student_username}}</a>"
body contains "/user/{{student_id}}/task/task1#record-2"
body contains "checkedTask=task1"
body contains "checkedTask=task2"
body contains "<span class=\"text-gray-200\">[all]</span>"
body matches "(?s)/user/{{student_id}}/task/task2\">Lab 2</a>\\s*</td>\\s*<td class=\"py-1 pl-2 text-right text-green-300\">\\s*<span class=\"text-gray-700\">—</span>\\s*</td>"
body matches "(?s)/user/{{student_id}}/task/task1\">Lab 1</a>\\s*</td>\\s*<td class=\"py-1 pl-2 text-right text-green-300\">\\s*7\\s*</td>"
body matches "(?s)/user/{{student_id}}/task/task2.*?/user/{{student_id}}/task/task1"

# Step 5: The date's link target exists on the task page
GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "id=\"record-2\""

# Step 6: Choosing one task hides reviews of the others and marks its button
GET {{base_url}}/user/{{teacher_id}}
Cookie: user_data={{teacher_token}}
[QueryStringParams]
checkedTask: task1

HTTP 200
[Asserts]
body contains "/user/{{student_id}}/task/task1\">Lab 1</a>"
body not contains "/user/{{student_id}}/task/task2\">Lab 2</a>"
body contains "<span class=\"text-gray-200\">[Lab 1]</span>"
body not contains "<span class=\"text-gray-200\">[all]</span>"

# Step 7: A student's page has no table, even when a teacher views it
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "// checked tasks"

GET {{base_url}}/logout
HTTP 303

GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "// checked tasks"

# Step 8: A student cannot open the teacher's page
GET {{base_url}}/user/{{teacher_id}}
Cookie: user_data={{student_token}}

HTTP 403
