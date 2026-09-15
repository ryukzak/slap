# Lesson Sort Modes Test
# The lesson page offers four ways to order task records: submit-ord,
# task-mix, student-mix, register-ord. submit-ord/task-mix/student-mix are
# all based on submit time (when the work was originally written), which
# never changes across register/reviewed/revoke transitions. register-ord is
# the only mode based on registration time (when each record was registered
# to *this* lesson) — a fix, since it used to reuse the same submit-time key
# as everything else and was indistinguishable from submit-ord.
#
# This test submits two students' two tasks in one order, then registers
# them to the same lesson in the reverse order, so submit time and
# registration time disagree — letting each mode's actual ordering be
# checked unambiguously via the unique `data-record-id="<student>:<task>"`
# marker each row carries.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)33 \
#   tests/ui/lesson-sort-modes.hurl

# Step 1: Sign in as teacher, create an unlimited-capacity lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_a_password=StudentAPass123!
variable: student_a_username=SortAlice
variable: student_b_password=StudentBPass123!
variable: student_b_username=SortBob
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
time: 17:00
description: Sort modes test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Submit in this order: Alice-task1, Bob-task1, Alice-task2, Bob-task2
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_a_password}}
username: {{student_a_username}}

HTTP 303
[Captures]
student_a_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Alice task1 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/signup
[FormParams]
id: {{student_b_id}}
password: {{student_b_password}}
username: {{student_b_username}}

HTTP 303
[Captures]
student_b_token: cookie "user_data"

POST {{base_url}}/user/{{student_b_id}}/task/task1/journal
Cookie: user_data={{student_b_token}}
[FormParams]
content: Bob task1 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Alice task2 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/user/{{student_b_id}}/task/task2/journal
Cookie: user_data={{student_b_token}}
[FormParams]
content: Bob task2 submission

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Register to the same lesson in the REVERSE order:
# Bob-task2, Alice-task2, Bob-task1, Alice-task1
POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_b_token}}
[FormParams]
taskRecordID: task2
studentID: {{student_b_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_a_token}}
[FormParams]
taskRecordID: task2
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_b_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_b_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_a_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

# Step 4: submit-ord follows submit time: Alice-T1, Bob-T1, Alice-T2, Bob-T2
GET {{base_url}}/lesson/{{lesson_id}}?sort=submit-ord
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body matches "(?s)data-record-id=\"{{student_id}}:task1\".*data-record-id=\"{{student_b_id}}:task1\".*data-record-id=\"{{student_id}}:task2\".*data-record-id=\"{{student_b_id}}:task2\""

# Step 5: register-ord follows registration time, the reverse:
# Bob-T2, Alice-T2, Bob-T1, Alice-T1
GET {{base_url}}/lesson/{{lesson_id}}?sort=register-ord
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body matches "(?s)data-record-id=\"{{student_b_id}}:task2\".*data-record-id=\"{{student_id}}:task2\".*data-record-id=\"{{student_b_id}}:task1\".*data-record-id=\"{{student_id}}:task1\""

# Step 6: task-mix interleaves the submit-order list by task: Alice-T1,
# Alice-T2, Bob-T1, Bob-T2 (would be Bob-T2,Bob-T1,Alice-T2,Alice-T1 if it
# were wrongly built on register order instead)
GET {{base_url}}/lesson/{{lesson_id}}?sort=task-mix
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body matches "(?s)data-record-id=\"{{student_id}}:task1\".*data-record-id=\"{{student_id}}:task2\".*data-record-id=\"{{student_b_id}}:task1\".*data-record-id=\"{{student_b_id}}:task2\""

# Step 7: student-mix interleaves the submit-order list by student, which
# for this input equals submit order: Alice-T1, Bob-T1, Alice-T2, Bob-T2
# (would be Bob-T2,Alice-T2,Bob-T1,Alice-T1 if it were wrongly built on
# register order instead)
GET {{base_url}}/lesson/{{lesson_id}}?sort=student-mix
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body matches "(?s)data-record-id=\"{{student_id}}:task1\".*data-record-id=\"{{student_b_id}}:task1\".*data-record-id=\"{{student_id}}:task2\".*data-record-id=\"{{student_b_id}}:task2\""

# Step 8: the sort-mode picker lists options in the order submit-ord,
# task-mix, student-mix, register-ord
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body matches "(?s)submit-ord.*task-mix.*student-mix.*register-ord"

GET {{base_url}}/logout
HTTP 303
