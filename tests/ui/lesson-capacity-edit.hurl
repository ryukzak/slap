# Lesson capacity edit Test
# The lesson's teacher can change the capacity after the lesson was created:
#   1. The lesson page shows an "edit capacity" form for the owning teacher.
#   2. POST /api/lesson/{id}/capacity updates the displayed capacity.
#   3. Capacity 0 means unlimited.
#   4. A negative capacity is rejected; non-teachers cannot change it.
#
# Prerequisites: server running with default config (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/lesson-capacity-edit.hurl

# Step 1: Sign in as teacher (signed up by earlier tests)
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Capacity Edit Student
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

# Step 2: Teacher creates a lesson with capacity 1
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 10:00
description: Lesson capacity edit test
capacity: 1

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

# Step 3: Lesson page shows capacity 1 and the edit form for the owner
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "lesson-detail-capacity\">1</span>"
body contains "/api/lesson/{{lesson_id}}/capacity"
body contains "[set capacity]"

# Step 4: Teacher raises the capacity to 5
POST {{base_url}}/api/lesson/{{lesson_id}}/capacity
Cookie: user_data={{teacher_token}}
[FormParams]
capacity: 5

HTTP 303

GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "lesson-detail-capacity\">5</span>"

# Step 5: Capacity 0 clears the limit
POST {{base_url}}/api/lesson/{{lesson_id}}/capacity
Cookie: user_data={{teacher_token}}
[FormParams]
capacity: 0

HTTP 303

GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "lesson-detail-capacity\">unlimited</span>"

# Step 6: A negative capacity is rejected
POST {{base_url}}/api/lesson/{{lesson_id}}/capacity
Cookie: user_data={{teacher_token}}
[FormParams]
capacity: -3

HTTP 400

GET {{base_url}}/logout
HTTP 303

# Step 7: Sign up a student — students cannot change the capacity
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

POST {{base_url}}/api/lesson/{{lesson_id}}/capacity
Cookie: user_data={{student_token}}
[FormParams]
capacity: 42

HTTP 403

# Step 8: The student does not see the capacity form on the lesson page
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "/api/lesson/{{lesson_id}}/capacity"
