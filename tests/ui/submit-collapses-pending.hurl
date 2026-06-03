# Submit Collapses Pending Test
# Verifies that the older pending record is collapsed to Dropped when a student
# adds a new submission while an older one is pending. After the second
# submission the task page must show a Pending record (the new one) and at least
# one Dropped (the old one).
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/submit-collapses-pending.hurl

# Step 1: Sign up student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: student_username=CollapseStudent
variable: task_id=task1
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

# Step 2: Student submits first version
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: First pending submission

HTTP 303

# Step 3: Student submits second version (first must get Dropped status)
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Second submission

HTTP 303

# Task page shows new submission as Pending and the old one as Dropped
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Pending"
body contains "Dropped"
