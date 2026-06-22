# User List Note Column Test
# Verifies that the /users table shows the latest teacher note for a student,
# and that the note is NOT present in the CSV export.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/user-list-note-column.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Note Column Student
variable: note_old=NOTECOL_OLD first note
variable: note_latest=NOTECOL_LATEST second note
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Sign up student
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Teacher adds an older note
POST {{base_url}}/user/{{student_id}}/note
Cookie: user_data={{teacher_token}}
[FormParams]
note: {{note_old}}

HTTP 303

# Step 4: Teacher adds the latest note
POST {{base_url}}/user/{{student_id}}/note
Cookie: user_data={{teacher_token}}
[FormParams]
note: {{note_latest}}

HTTP 303

# Step 5: /users table shows the latest note, not the older one
GET {{base_url}}/users
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "{{note_latest}}"
body not contains "{{note_old}}"

# Step 6: CSV export does NOT include any note content
GET {{base_url}}/users/csv
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "{{note_latest}}"
body not contains "{{note_old}}"
