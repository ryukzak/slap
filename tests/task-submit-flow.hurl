# Task Submission Flow Test
# Tests: student signup → submit task → view history → submit second record
#
# Prerequisites: server running against a fresh DB with default config.
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   tests/task-submit-flow.hurl

# Step 1: Sign up as student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: student_username=Test Student
variable: task_id=task1
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
session_cookie: cookie "user_data"
[Asserts]
header "Location" == "/user/{{student_id}}"

# Step 2: View task page (no records yet)
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{session_cookie}}

HTTP 200
[Asserts]
body contains "No records for this task yet"

# Step 3: Submit a task record
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{session_cookie}}
[FormParams]
content: My first submission with **markdown**

HTTP 303
[Asserts]
header "Location" == "/user/{{student_id}}/task/{{task_id}}"

# Step 4: Verify record appears on task page
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{session_cookie}}

HTTP 200
[Asserts]
body contains "My first submission with"
body contains "{{student_username}}"

# Step 5: Submit a second record
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{session_cookie}}
[FormParams]
content: My second submission with more detail

HTTP 303

# Step 6: Verify both records on task page (newest first)
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{session_cookie}}

HTTP 200
[Asserts]
body contains "My second submission with more detail"
body contains "My first submission with"

# Step 7: Dashboard shows task with Pending status badge
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{session_cookie}}

HTTP 200
[Asserts]
body contains "Pending"
