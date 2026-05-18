# Re-registration after revoke Test
# Covers three scenarios:
# 1. Task page shows Pending (not Dropped) after revoke
# 2. Teacher's lesson page shows Dropped status in history after revoke by a student
# 3. Student can re-register the same task to a different lesson after revoke
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/reregistration-after-revoke.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Re-registrated Student
variable: task_id=task1
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"



# Step 2: Teacher creates lesson A
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 10:00
description: Lesson A revoke test

HTTP 200
[Captures]
lesson_a_id: url regex "/lesson/([^/?#]+)"

# Step 3: Teacher creates lesson B
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 12:00
description: Lesson B revoke test

HTTP 200
[Captures]
lesson_b_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 4: Sign up student
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 5: Student submits task
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Submission for revoke test

HTTP 303

# Step 6: Student registers task to lesson A
POST {{base_url}}/api/lesson/{{lesson_a_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Step 7: Student revokes from lesson A
POST {{base_url}}/api/lesson/{{lesson_a_id}}/unregister
Cookie: user_data={{student_token}}
[FormParams]
taskID: {{task_id}}

HTTP 303

# Scenario 1: task page shows Pending, not Dropped
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Pending"
body not contains "Dropped"

# Scenario 2: teacher's lesson A page shows Dropped in history
GET {{base_url}}/lesson/{{lesson_a_id}}?showRevoked=true
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Dropped"

# Scenario 3: student re-registers same task to lesson B
POST {{base_url}}/api/lesson/{{lesson_b_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Verify task is now Queued for lesson B
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Queued"
