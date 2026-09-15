# Dashboard Lesson Queue Display Test
# Both the dashboard's "// lessons" list and a task page's "// register"
# section used to show queue info as a bare three-number bracket like
# "[0/1/5]" (reviewed/registered/capacity), which reads like a single
# confusing fraction. It now reads as
# "<registered>/<capacity> queued (<reviewed> checked)", or just
# "<registered> queued (<reviewed> checked)" when the lesson has no capacity
# limit (capacity 0 means unlimited).
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)32 \
#   tests/ui/dashboard-lesson-queue-display.hurl

# Step 1: Sign in as teacher, create a capacity-limited lesson and an
# unlimited one
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=QueueDisplayStudent
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 15:00
description: Queue display test lesson (limited)
capacity: 5

HTTP 200
[Captures]
lesson_limited_id: url regex "/lesson/([^/?#]+)"

POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 16:00
description: Queue display test lesson (unlimited)

HTTP 200
[Captures]
lesson_unlimited_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits task1 and task2, registers each to a lesson
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Submission for limited lesson

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_limited_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Submission for unlimited lesson

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_unlimited_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task2
studentID: {{student_id}}

HTTP 200

# Step 3: Before any review, the dashboard shows both queued, none checked
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "1/5 queued (0 checked)"
body contains "1 queued (0 checked)"
body not contains "[reviewed/registered/capacity]"

# Step 3b: The task page's own "// register" section (shown while still
# queued) uses the same wording
GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "1/5 queued (0 checked)"
body not contains "[reviewed/registered/capacity]"

GET {{base_url}}/logout
HTTP 303

# Step 4: Teacher reviews the limited-lesson submission
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: Looks good

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 5: The dashboard now shows that lesson as checked, the other still not
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "1/5 queued (1 checked)"
body contains "1 queued (0 checked)"

GET {{base_url}}/logout
HTTP 303
