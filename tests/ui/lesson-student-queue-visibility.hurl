# Lesson Student Queue Visibility Test
# Tests: student can see other students' entries in the lesson queue,
#        but cannot see the submitted content of other students.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_a_password=StudentAPass123!
variable: student_a_username=AliceQueue
variable: student_b_password=StudentBPass123!
variable: student_b_username=BobQueue
variable: student_b_id={{student_id}}1
variable: task_id=task1
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Teacher creates a lesson
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 14:00
description: Queue visibility test lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

# Step 3: Sign up student A
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_a_password}}
username: {{student_a_username}}

HTTP 303
[Captures]
student_a_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 4: Student A submits and registers a task
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_a_token}}
[FormParams]
content: Alice secret submission content

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_a_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Step 5: Sign up student B
POST {{base_url}}/signup
[FormParams]
id: {{student_b_id}}
password: {{student_b_password}}
username: {{student_b_username}}

HTTP 303
[Captures]
student_b_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 6: Student B submits and registers a task
POST {{base_url}}/user/{{student_b_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_b_token}}
[FormParams]
content: Bob secret submission content

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_b_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_b_id}}

HTTP 200

# Step 7: Student A views lesson page - sees both students in queue
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_a_token}}

HTTP 200
[Asserts]
body contains "@AliceQueue"
body contains "@BobQueue"
body contains "Queued"
# Student A sees own submission content
body contains "Alice secret submission content"
# Student A does NOT see student B's submission content
body not contains "Bob secret submission content"
# Student A sees placeholder for other's content
body contains "Submission content is only visible to the author and teacher."

# Step 8: Student B views lesson page - sees both students but not A's content
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_b_token}}

HTTP 200
[Asserts]
body contains "@AliceQueue"
body contains "@BobQueue"
body contains "Queued"
# Student B sees own submission content
body contains "Bob secret submission content"
# Student B does NOT see student A's submission content
body not contains "Alice secret submission content"

# Step 9: Teacher still sees all content
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "@AliceQueue"
body contains "@BobQueue"
body contains "Alice secret submission content"
body contains "Bob secret submission content"
