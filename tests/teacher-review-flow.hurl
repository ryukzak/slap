# Teacher Review Flow Test
# Tests: teacher signs in → views student task → submits review →
#        student sees feedback
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/teacher-review-flow.hurl

# Step 1: Sign up student — capture cookie from 303 response
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: student_username=Review Student
variable: teacher_password=TeacherPass123!
variable: teacher_username=Test Teacher
variable: task_id=task1
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

# Step 2: Student submits a task record
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Student solution ready for review

HTTP 303

# Step 3: Sign up teacher (ID must match config teacher_ids)
POST {{base_url}}/signup
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}
username: {{teacher_username}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"
[Asserts]
header "Location" == "/user/{{teacher_id}}"

# Step 4: Teacher views student task page
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Student solution ready for review"

# Step 5: Teacher submits review feedback
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: Good work! A few things to improve.

HTTP 303
[Asserts]
header "Location" == "/user/{{student_id}}/task/{{task_id}}"

# Step 6: Teacher sees both records on task page
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Good work! A few things to improve."
body contains "Student solution ready for review"
body contains "Feedback"

# Step 7: Student also sees teacher feedback
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Good work! A few things to improve."
body contains "Feedback"

# Step 8: Teacher submits a second review
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: One more thing to address.

HTTP 303

# Step 9: Both reviews are visible on the task page
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Good work! A few things to improve."
body contains "One more thing to address."
