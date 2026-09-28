# Course Activity Heatmap Test
# The teacher dashboard (/users) now has a second, separate collapsible
# section below "// activity": "// course activity", a per-task heatmap
# (one row per task, one cell per week spanning the whole course) showing
# every student's activity, not just one day's lessons. Every cell shares
# one color (blue); only its shade varies, interpolated pale-to-saturated by
# volume relative to the busiest cell -- fully opaque, so the gradient reads
# in both light and dark themes. Record type (submit/register/reviewed/
# revoke) no longer maps to hue -- a mixed week would otherwise collapse to
# one arbitrarily "dominant" color and hide the rest -- so the type
# breakdown lives in the cell's hover tooltip instead.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)42 \
#   tests/ui/course-activity-heatmap.hurl

# Step 1: Sign in as teacher, create a lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=HeatmapStudent
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
description: Heatmap test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits and registers into the lesson, teacher reviews
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: My topic

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task2
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: 8 Nice work

HTTP 303

# Step 3: Teacher dashboard shows the new heatmap section, with task2's row,
# a solid-blue cell (it's the only/busiest cell -> fully saturated), and its
# hover tooltip carrying the type breakdown
GET {{base_url}}/users
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "// course activity"
body contains "Lab 2"
body contains "background-color: rgb(37,99,235)"
body contains "submitted"
body contains "queued"
body contains "checked"
body contains "shade = volume that week"

GET {{base_url}}/logout
HTTP 303
