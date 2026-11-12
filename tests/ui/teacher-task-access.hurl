# Teacher Task Access Test
# `teacher_tasks` in the config limits which tasks a teacher checks. Teacher
# 12345 has task2 black-listed: it cannot review task2 (the task page hides the
# feedback form and the POST is rejected), and students cannot register task2
# to that teacher's lessons (the lesson is hidden from the register list and the
# POST is rejected). task1 stays fully available.
#
# Prerequisites: server running with conf/config.yaml
#   (teacher_tasks: {"12345": {deny: [task2]}}).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable student_id=$(date +%s)52 \
#   tests/ui/teacher-task-access.hurl

# Step 1: Sign up teacher 12345 (may already exist on a reused DB), then sign in
POST {{base_url}}/signup
[Options]
variable: limited_teacher_id=12345
variable: limited_teacher_password=LimitedTeacher123!
variable: student_password=AccessPass123!
[FormParams]
id: {{limited_teacher_id}}
password: {{limited_teacher_password}}
username: Limited Teacher

HTTP *

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/signin
[FormParams]
id: {{limited_teacher_id}}
password: {{limited_teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

# Step 2: Teacher creates a lesson
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 10:00
description: Teacher task access test

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 3: Student signs up and submits task1 and task2
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: Access Student

HTTP 303
[Captures]
student_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Access task1 submission

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Access task2 submission

HTTP 303

# Step 4: Register list offers the lesson for task1 but not for task2
GET {{base_url}}/api/lessons?register=1&task_id=task1&student_id={{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "/api/lesson/{{lesson_id}}/register"

GET {{base_url}}/api/lessons?register=1&task_id=task2&student_id={{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "/api/lesson/{{lesson_id}}/register"

# Step 5: Registering task2 to the lesson is rejected; task1 is accepted
POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task2
studentID: {{student_id}}

HTTP 403

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: task1
studentID: {{student_id}}

HTTP 200

GET {{base_url}}/logout
HTTP 303

# Step 6: Teacher sees no feedback form on task2, but has one on task1
GET {{base_url}}/user/{{student_id}}/task/task2
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "You are not allowed to check this task."
body not contains "/user/{{student_id}}/task/task2/journal"

GET {{base_url}}/user/{{student_id}}/task/task1
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "You are not allowed to check this task."
body contains "/user/{{student_id}}/task/task1/journal"

# Step 7: Reviewing task2 is rejected; reviewing task1 works
POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: Should not be saved score:5

HTTP 403

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: Looks good score:5

HTTP 303

GET {{base_url}}/user/{{student_id}}/task/task2
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "Should not be saved"
