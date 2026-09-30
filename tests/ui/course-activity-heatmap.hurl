# Course Activity Heatmap Test
# The teacher dashboard (/users) now has a second, separate collapsible
# section below "// activity": "// course activity", a per-task heatmap
# (one row per task, one week per column, spanning the whole course) showing
# every student's activity, not just one day's lessons. Each week renders
# two Unicode shade-block glyphs (░▒▓█) -- submissions on the left, teacher
# reviews on the right -- each independently bucketed by its own count, not
# scaled against any page-wide max; queued/dropped counts stay in the hover
# tooltip only. "step" (selector: 1/2/5/10/25, default 2) is how many
# counts each glyph level spans -- e.g. at step 2, 1-2 submits is the
# lightest glyph, 3-4 the next, and so on, capping at the densest glyph.
# Buckets are fixed at one week wide. The date range above the grid is a plain caption, not
# aligned to the grid's edges -- an earlier edge-aligned version misaligned
# once the grid's actual rendered width didn't match the label row's.
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
# a lightest-glyph submit and a lightest-glyph check (1 each, at the default
# step of 2), the step selector, and the plain (non-edge-aligned) date range
# caption
GET {{base_url}}/users
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "// course activity"
body contains "Lab 2"
body contains "one pair of symbols per week"
body contains "submitted"
body contains "queued"
body contains "checked"
body contains "step:"
body contains ">1<"
body contains ">5<"
body contains ">10<"
body contains ">25<"
# Glyph and range sit in separate adjacent <span>s, so check each span's
# own text content rather than assuming they're contiguous in the raw body.
body contains ">░<"
body contains ">1–2<"
body contains ">▒<"
body contains ">3–4<"
body contains ">▓<"
body contains ">5–6<"
body contains ">█<"
body contains ">≥7<"

GET {{base_url}}/logout
HTTP 303
