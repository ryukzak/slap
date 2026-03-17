# Settings Flow Test
# Tests: change display name → verify on profile; change password → old fails, new works
#
# Prerequisites: server running against a fresh DB with default config.
# Run: hurl --test \
#   --variable student_id=student_$(date +%s) \
#   tests/settings-flow.hurl

# Step 1: Sign up student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: new_password=NewStudentPass456!
variable: student_username=Settings Student
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

# Step 2: Settings page is accessible
GET {{base_url}}/user/{{student_id}}/settings
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "// settings"
body contains "Settings Student"

# Step 3: Change display name
POST {{base_url}}/user/{{student_id}}/settings/username
Cookie: user_data={{student_token}}
[FormParams]
username: Renamed Student

HTTP 200
[Asserts]
body contains "Name updated"
body contains "Renamed Student"

# Step 4: Profile shows new name
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Renamed Student"

# Step 5: Change password with wrong current password → error
POST {{base_url}}/user/{{student_id}}/settings/password
Cookie: user_data={{student_token}}
[FormParams]
current_password: WrongPassword999!
new_password: {{new_password}}

HTTP 200
[Asserts]
body contains "Current password is incorrect"

# Step 6: Change password with correct current password → success
POST {{base_url}}/user/{{student_id}}/settings/password
Cookie: user_data={{student_token}}
[FormParams]
current_password: {{student_password}}
new_password: {{new_password}}

HTTP 200
[Asserts]
body contains "Password updated"

# Step 7: Old password no longer works
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: {{student_password}}

HTTP 200
[Asserts]
body contains "Invalid ID or password"

# Step 8: New password works
POST {{base_url}}/signin
[FormParams]
id: {{student_id}}
password: {{new_password}}

HTTP 303
[Asserts]
header "Location" == "/user/{{student_id}}"

# Step 9: Another user cannot access settings
POST {{base_url}}/signup
[FormParams]
id: other_{{student_id}}
password: OtherPass123!
username: Other Student

HTTP 303
[Captures]
other_token: cookie "user_data"

GET {{base_url}}/user/{{student_id}}/settings
Cookie: user_data={{other_token}}

HTTP 403
