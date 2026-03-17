# Access Control Test
# Tests: unauthenticated access blocked, student cannot view other student's
#        profile, student cannot create lessons, teacher can view any profile
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_a_id=student_a_$(date +%s) \
#   --variable student_b_id=student_b_$(date +%s) \
#   --variable teacher_id=123 \
#   tests/access-control.hurl

# Rule 1: Unauthenticated request to profile returns 401
# (cookie jar is empty — no /set-cookie calls used in this file)
GET {{base_url}}/user/{{student_a_id}}

HTTP 401
[Asserts]
body contains "Authentication required"

# Rule 2: Unauthenticated request to task page returns 401
GET {{base_url}}/user/{{student_a_id}}/task/task1

HTTP 401

# Setup: sign up student A — capture cookie from 303 response
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: teacher_password=TeacherPass123!
[FormParams]
id: {{student_a_id}}
password: {{student_password}}
username: Student A

HTTP 303
[Captures]
student_a_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Setup: sign up student B
POST {{base_url}}/signup
[FormParams]
id: {{student_b_id}}
password: {{student_password}}
username: Student B

HTTP 303
[Captures]
student_b_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Setup: sign in as teacher (already registered from earlier test)
POST {{base_url}}/signin
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Rule 3: Student A cannot view student B's profile
GET {{base_url}}/user/{{student_b_id}}
Cookie: user_data={{student_a_token}}

HTTP 403
[Asserts]
body contains "Forbidden"

# Rule 4: Student A cannot view student B's task
GET {{base_url}}/user/{{student_b_id}}/task/task1
Cookie: user_data={{student_a_token}}

HTTP 403

# Rule 5: Student cannot create a lesson
POST {{base_url}}/api/lessons
Cookie: user_data={{student_a_token}}
[FormParams]
date: 2099-12-31
time: 14:00
description: Student trying to create lesson

HTTP 403
[Asserts]
body contains "Only teachers can do that"

# Rule 6: Student cannot access user list
GET {{base_url}}/users
Cookie: user_data={{student_a_token}}

HTTP 403

# Rule 7: Student can view their own profile
GET {{base_url}}/user/{{student_a_id}}
Cookie: user_data={{student_a_token}}

HTTP 200
[Asserts]
body contains "Student A"

# Rule 8: Teacher can view student A's profile
GET {{base_url}}/user/{{student_a_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Student A"

# Rule 9: Teacher can view student B's profile
GET {{base_url}}/user/{{student_b_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Student B"

# Rule 10: Teacher can access user list
GET {{base_url}}/users
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Student A"
body contains "Student B"
