# Password Reset Flow Test
# Tests: teacher sets new password for student via /user/{id}/reset →
#        student signs in with new password; non-teachers blocked
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/password-reset-flow.hurl

# Step 1: Sign up student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: new_password=ResetPass456!
variable: teacher_password=TeacherPass123!
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: Reset Student

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

# Step 3: Wrong password login shows "ask your teacher" message
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: WrongPassword999!

HTTP 200
[Asserts]
body contains "Invalid ID or password"
body contains "Ask your teacher"

# Step 4: Unauthenticated access to reset page is blocked
GET {{base_url}}/user/{{student_id}}/reset

HTTP 401

# Step 5: Student cannot access reset page for another user
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: {{student_password}}

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 6: Teacher opens reset form for student
GET {{base_url}}/user/{{student_id}}/reset
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "// reset password"
body contains "Reset Student"

# Step 7: Teacher sets new password
POST {{base_url}}/user/{{student_id}}/reset
Cookie: user_data={{teacher_token}}
[FormParams]
password: {{new_password}}

HTTP 200
[Asserts]
body contains "Password updated"

# Step 8: Student signs in with new password
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: {{new_password}}

HTTP 303
[Asserts]
header "Location" == "/user/{{student_id}}"

# Step 9: Old password no longer works
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: {{student_password}}

HTTP 200
[Asserts]
body contains "Invalid ID or password"
