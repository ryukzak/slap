# Lesson Summary Refresh Test
# A review submitted from the lesson page is sent by htmx, and the handler
# answers 204 + "HX-Trigger: lessonRecordsRefresh". htmx dispatches that event
# on the triggering element — the review form — so it only bubbles up to
# ancestors. The queue list (#lesson-task-records) contains the form and
# refreshes; the summary panel (#lesson-stats) is a sibling section, so it
# never hears the event and keeps showing pre-review counts until a manual
# reload. The panel must therefore listen on the body ("from:body").
#
# hurl runs no JavaScript, so this test pins the two halves the server owns:
# the panel is wired to a body-scoped trigger, and the stats partial that the
# refresh fetches reports the new check.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)43 \
#   tests/ui/lesson-summary-refresh.hurl

# Step 1: Sign in as teacher, create a lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=RefreshPass123!
variable: student_username=SummaryRefresh
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
time: 18:00
description: Summary refresh test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits task1 and registers it to the lesson
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
content: Summary refresh submission

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

# Step 3: The summary panel listens for the refresh event on the body, so an
# htmx review posted from the queue list below it still reaches the panel.
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body matches "(?s)id=\"lesson-stats\"[^>]*hx-trigger=\"lessonRecordsRefresh from:body\""
body contains "checked: 0"
body contains "queued: 1"

# Step 4: Teacher reviews from the lesson page the way the form does: an htmx
# request, answered with no content and the refresh trigger.
POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
HX-Request: true
[FormParams]
role: review
content: 8 solid work

HTTP 204
[Asserts]
header "HX-Trigger" == "lessonRecordsRefresh"

# Step 5: The stats partial the refresh fetches reports the new check.
GET {{base_url}}/lesson/{{lesson_id}}/stats
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "checked: 1"
body contains "queued: 0"
body contains "min:8"
body contains "@Test Teacher"

GET {{base_url}}/logout
HTTP 303
