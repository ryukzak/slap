# Teacher Note Test
# Verifies teacher notes for a student:
#   1. Any teacher can add notes; notes are an append-only list with author.
#   2. The student page shows all notes (with author + count), teachers only.
#   3. The student never sees the notes on their own page.
#   4. The lesson page shows only the last note text (no author) plus a count,
#      teachers only.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/teacher-note.hurl

# Step 1: Sign in as teacher (already registered from earlier tests)
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=Teacher Note Student
variable: task_id=task1
variable: note_one=NOTEMARKERONE strong on recursion
variable: note_two=NOTEMARKERTWO weak on testing
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Sign up student
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

# Step 3: Teacher adds the first note
POST {{base_url}}/user/{{student_id}}/note
Cookie: user_data={{teacher_token}}
[FormParams]
note: {{note_one}}

HTTP 303

# Step 4: Teacher adds the second note
POST {{base_url}}/user/{{student_id}}/note
Cookie: user_data={{teacher_token}}
[FormParams]
note: {{note_two}}

HTTP 303

# Step 5: Teacher sees all notes (with author + count) on the student page
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "teacher notes (2)"
body contains "{{note_one}}"
body contains "{{note_two}}"
body contains "@Test Teacher"

# Step 6: Student does NOT see the notes on their own page
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "{{note_one}}"
body not contains "{{note_two}}"

# Step 7: Teacher creates a lesson
POST {{base_url}}/api/lessons
Cookie: user_data={{teacher_token}}
[Options]
location: true
[FormParams]
date: 2099-12-31
time: 14:00
description: Teacher note lesson

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

# Step 8: Student submits a task record
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Submission for teacher note test

HTTP 303

# Step 9: Student registers task to lesson
POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Step 10: Teacher sees only the last note + count on the lesson page
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "teacher note (2)"
body contains "{{note_two}}"
body not contains "{{note_one}}"

# Step 11: Student does NOT see the notes on the lesson page
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "{{note_one}}"
body not contains "{{note_two}}"
