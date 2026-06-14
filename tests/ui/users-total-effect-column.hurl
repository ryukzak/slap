# UI Test: Users Table Effect Column
# Verifies that the users table shows the Effect/Bonus-Penalty column
# and displays correct total effect for students.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"], score_rules configured).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/users-total-effect-column.hurl

# Step 1: Sign up student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: teacher_password=TeacherPass123!
variable: student_username=Effect Test Student
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

# Step 3: Teacher submits a review with score for task1 (penalty will apply)
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: 8 - Good work but late

HTTP 303

# Step 4: Teacher views users table
GET {{base_url}}/users
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Effect"
body contains "Bonus/Penalty"
body contains "{{student_id}}"
body contains "-2"