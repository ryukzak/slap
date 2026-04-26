# Score Rules Test
# Tests: after deadline penalty, before deadline bonus, min_checked_before group penalty
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"], score_rules configured).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/score-rules.hurl

# Step 1: Sign up student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: teacher_password=TeacherPass123!
variable: student_username=ScoreRules Student
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Sign in as teacher
POST {{base_url}}/signin
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Test 1: After deadline penalty (Lab 1 hard deadline)

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Lab 1 submission after deadline

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: 8 - Good work but late

HTTP 303

GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Lab 1 hard deadline"
body contains "applied"
body contains "-2"

# Test 2: Before deadline bonus (Early bird Lab 4)

POST {{base_url}}/user/{{student_id}}/task/task4/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Early submission for bonus

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task4/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: 10 - Excellent early work

HTTP 303

GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Early bird Lab 4"
body contains "+1"

# Test 3: Group penalty - less than N before deadline

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Lab 1 submission

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: 8 - Good

HTTP 303

GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Mid-semester checkpoint"
body contains "-1"

# Test 4: CSV export includes Bonus/Penalty column

GET {{base_url}}/users/csv
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
header "Content-Type" contains "text/csv"
body contains "Bonus/Penalty"

# Test 5: Student profile shows total effect

GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "total effect:"