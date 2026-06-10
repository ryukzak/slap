# Re-registration after revoke Test (append/resubmit approach)
# Covers three scenarios:
# 1. After revoke the task page keeps the Dropped record AND shows a fresh
#    Pending resubmission (transparent history, not a silent status flip).
# 2. Teacher's lesson page shows Dropped status in history after revoke.
# 3. Student can re-register the same task to a different lesson after revoke
#    without resubmitting (the pending resubmission becomes Queued).
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
variable: student_username=Resubmit Student
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
description: Lesson A resubmit test

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
description: Lesson B resubmit test

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
content: Submission for resubmit test

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

# Scenario 1: task page keeps the Dropped record and shows a fresh Pending one.
# The pending resubmission must be treated as the latest record, so the lesson
# registration list is shown right away (the student keeps their queue position
# and can re-register without submitting again).
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Pending"
body contains "Dropped"
body contains "/api/lessons?register=1"

# The lesson list the task page loads (via htmx) must offer lessons to
# register to, right after the revoke.
GET {{base_url}}/api/lessons?register=1&task_id={{task_id}}&student_id={{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Lesson B resubmit test"
body contains "[register]"
body contains "/api/lesson/{{lesson_b_id}}/register"

# Scenario 2: teacher's lesson A page shows Dropped in history
GET {{base_url}}/lesson/{{lesson_a_id}}?showRevoked=true
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Dropped"

# Scenario 3: student re-registers same task to lesson B (no resubmit needed)
POST {{base_url}}/api/lesson/{{lesson_b_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Verify task is now Queued for lesson B (and the drop is still visible)
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Queued"
body contains "Dropped"
