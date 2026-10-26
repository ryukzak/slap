# Task variant display Test
# The config's variants_file (conf/variants.csv) assigns students a variant per
# task. Student 900001 has variant 7 for task1 and none for task2.
#   1. The task page shows the student's variant, and only when one is assigned.
#   2. The lesson page shows the variant in the student's queue row details
#      (next to submitted/registered) to the teacher.
#   3. A student without an assigned variant sees no variant section.
#
# Prerequisites: server running with default config (teacher_ids: ["123"],
# variants_file: variants.csv).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/task-variant-display.hurl

# Step 1: Sign up the student listed in conf/variants.csv. The id is fixed, so
# on a reused database the account may already exist — then sign in instead.
POST {{base_url}}/signup
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: variant_student_id=900001
[FormParams]
id: {{variant_student_id}}
password: {{student_password}}
username: Variant Student

HTTP *

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/signin
[FormParams]
id: {{variant_student_id}}
password: {{student_password}}

HTTP 303
[Captures]
variant_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: The task page shows the assigned variant
GET {{base_url}}/user/{{variant_student_id}}/task/task1
Cookie: user_data={{variant_token}}

HTTP 200
[Asserts]
body contains "// variant"
body contains "task-variant\">7</span>"

# Step 3: No variant section for a task with an empty cell
GET {{base_url}}/user/{{variant_student_id}}/task/task2
Cookie: user_data={{variant_token}}

HTTP 200
[Asserts]
body not contains "task-variant"

# Step 4: Teacher creates a lesson and the student registers task1
POST {{base_url}}/signin
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 12:00
description: Variant display test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

POST {{base_url}}/user/{{variant_student_id}}/task/task1/journal
Cookie: user_data={{variant_token}}
[FormParams]
content: Variant solution

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{variant_token}}
[FormParams]
taskRecordID: task1
studentID: {{variant_student_id}}

HTTP 200

# Step 5: The teacher sees the variant in the queue row's details
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@Variant Student"
body contains "lesson-record-variant text-gray-300\">7</span>"

# Step 6: The teacher also sees it on the student's task page
GET {{base_url}}/user/{{variant_student_id}}/task/task1
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "task-variant\">7</span>"

# Step 7: Delete the lesson so a rerun on the same database can register again
DELETE {{base_url}}/api/lessons/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200

# Step 8: A student without an assigned variant sees no variant section
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: No Variant Student

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "task-variant"
