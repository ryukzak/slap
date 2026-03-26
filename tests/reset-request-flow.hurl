# Reset Request Flow Test
# Tests: student requests reset link → shares with teacher → teacher resets
#        password via token link → student signs in with new password;
#        reused token rejected after reset
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/reset-request-flow.hurl

# Step 1: Sign up student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: new_password=TokenReset789!
variable: teacher_password=TeacherPass123!
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: Token Student

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 2: Sign in as teacher (ensures teacher account exists)
POST {{base_url}}/signin
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 3: GET /reset-request shows form
GET {{base_url}}/reset-request

HTTP 200
[Asserts]
body contains "// reset request"
body contains "Enter your user ID"
body contains "[request reset]"

# Step 4: POST /reset-request with non-existent user shows error
POST {{base_url}}/reset-request
[FormParams]
id: 99999999999

HTTP 200
[Asserts]
body contains "User not found"

# Step 5: Wrong password login shows reset request link
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: WrongPassword999!

HTTP 200
[Asserts]
body contains "Invalid ID or password"
body contains "/reset-request"
body contains "[request reset]"

# Step 6: POST /reset-request with valid student ID shows reset link
POST {{base_url}}/reset-request
[FormParams]
id: {{student_id}}

HTTP 200
[Asserts]
body contains "Share the link below"
body contains "reset link (valid 24h)"
body contains "/user/{{student_id}}/reset?token="
[Captures]
reset_token: body regex "/user/[0-9]+/reset\\?token=([0-9a-f]{64})"

# Step 7: Unauthenticated access to token reset link is blocked
GET {{base_url}}/user/{{student_id}}/reset?token={{reset_token}}

HTTP 401

# Step 8: Teacher opens token reset link → sees reset form
GET {{base_url}}/user/{{student_id}}/reset?token={{reset_token}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "// reset password"
body contains "Token Student"

# Step 9: Teacher sets new password via token link
POST {{base_url}}/user/{{student_id}}/reset?token={{reset_token}}
Cookie: user_data={{teacher_token}}
[FormParams]
password: {{new_password}}

HTTP 200
[Asserts]
body contains "Password updated"

# Step 10: Student signs in with new password
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: {{new_password}}

HTTP 303
[Asserts]
header "Location" == "/user/{{student_id}}"

GET {{base_url}}/logout
HTTP 303

# Step 11: Old password no longer works
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: {{student_password}}

HTTP 200
[Asserts]
body contains "Invalid ID or password"

# Step 12: Reusing the same token is rejected (token cleared on password update)
GET {{base_url}}/user/{{student_id}}/reset?token={{reset_token}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Invalid or expired reset link"

# Step 13: Teacher can still reset directly without token (existing flow)
GET {{base_url}}/user/{{student_id}}/reset
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "// reset password"
body contains "Token Student"
body not contains "Invalid or expired"
