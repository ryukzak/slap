# Tag Detail Queued Filter and Lesson Link Test
# The tag detail page (/tags/{tag}) now:
#   1. Shows a [lesson] link (between the Queued status and [task]) for
#      any row whose latest record is a lesson registration, linking to
#      /lesson/{lessonID}.
#   2. Offers a Queued filter pill that, when toggled on, hides every row
#      that isn't currently queued.
#   3. Enumerates every shown row with a leading #N, renumbered to the
#      currently visible rows (not the full unfiltered set).
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)39 \
#   tests/ui/tag-detail-queued-filter.hurl

# Step 1: Sign in as teacher, create a lesson to queue into
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_a_password=StudentAPass123!
variable: student_a_username=QueueTagAlice
variable: student_b_password=StudentBPass123!
variable: student_b_username=QueueTagBob
variable: student_b_id={{student_id}}1
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
description: Queued filter test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student A submits adhoctask1, teacher tags it while reviewing, then
# Student A submits again and registers to the lesson -- ends up Queued.
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_a_password}}
username: {{student_a_username}}

HTTP 303
[Captures]
student_a_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/adhoctask1/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Alice adhoctask1 first submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_id}}/task/adhoctask1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: Needs another pass \#queuedtag

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_id}}/task/adhoctask1/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Alice adhoctask1 second submission

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_a_token}}
[FormParams]
taskRecordID: adhoctask1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

# Step 3: Student B submits task3, teacher reviews and tags it -- stays
# Checked (not queued).
POST {{base_url}}/signup
[FormParams]
id: {{student_b_id}}
password: {{student_b_password}}
username: {{student_b_username}}

HTTP 303
[Captures]
student_b_token: cookie "user_data"

POST {{base_url}}/user/{{student_b_id}}/task/task3/journal
Cookie: user_data={{student_b_token}}
[FormParams]
content: Bob task3 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_b_id}}/task/task3/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: Good \#queuedtag

HTTP 303

# Step 4: Default view (no filter params) shows both rows, numbered #1 and
# #2, with a [lesson] link only on Alice's queued row.
GET {{base_url}}/tags/queuedtag
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@QueueTagAlice"
body contains "@QueueTagBob"
body contains "#1"
body contains "#2"
body contains "href=\"/lesson/{{lesson_id}}\""
body contains "href=\"?scored=true&amp;unscored=true&amp;queued=true\""

# Step 5: Turning the Queued pill on hides Bob's row and renumbers to just #1.
GET {{base_url}}/tags/queuedtag?scored=true&unscored=true&queued=true
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@QueueTagAlice"
body not contains "@QueueTagBob"
body contains "(1 of 2)"
body contains "#1"
body not contains "#2"
body contains "href=\"/lesson/{{lesson_id}}\""

GET {{base_url}}/logout
HTTP 303
