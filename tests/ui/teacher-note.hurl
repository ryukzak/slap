# Teacher Note Test
# Verifies teacher notes for a student:
#   1. Any teacher can add notes; notes are an append-only list with author.
#   2. Notes are never inline, so slap can be screen-shared: the student page
#      and the lesson page only carry a prompter hint with the note count.
#   3. The prompter notes partial shows all notes (text, author), teachers
#      only; adding a note returns to it.
#   4. The student never sees the notes or the hints.
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

# Step 3: Teacher adds two notes; each returns to the prompter partial
POST {{base_url}}/user/{{student_id}}/note
Cookie: user_data={{teacher_token}}
[FormParams]
note: {{note_one}}

HTTP 303
[Asserts]
header "Location" == "/prompter/user/{{student_id}}"

POST {{base_url}}/user/{{student_id}}/note
Cookie: user_data={{teacher_token}}
[FormParams]
note: {{note_two}}

HTTP 303

# Step 4: The student page shows a prompter hint, not the notes
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "{{note_one}}"
body not contains "{{note_two}}"
body contains "data-prompter-focus=\"{{student_id}}\""
body contains "[teacher notes (2): open prompter]"

# Step 5: The prompter shows all notes with authors
GET {{base_url}}/prompter
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "id=\"prompter\""

GET {{base_url}}/prompter/user/{{student_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "teacher notes (2)"
body contains "@{{student_username}}"
body contains "{{note_one}}"
body contains "{{note_two}}"
body contains "@Test Teacher"
body contains "hx-post=\"/user/{{student_id}}/note\""

# Step 6: The student sees neither the notes nor the prompter
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "{{note_one}}"
body not contains "open prompter"

GET {{base_url}}/prompter
Cookie: user_data={{student_token}}

HTTP 403

GET {{base_url}}/prompter/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 403

# Step 7: Teacher creates a lesson, student registers a task
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

POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Submission for teacher note test

HTTP 303

POST {{base_url}}/api/lesson/{{lesson_id}}/register
Cookie: user_data={{student_token}}
[FormParams]
taskRecordID: {{task_id}}
studentID: {{student_id}}

HTTP 200

# Step 8: The lesson page shows the hint and focus marker, not the notes
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "{{note_one}}"
body not contains "{{note_two}}"
body contains "data-prompter-student=\"{{student_id}}\""
body contains "[teacher notes (2): open prompter]"

GET {{base_url}}/lesson/{{lesson_id}}/records
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body not contains "{{note_two}}"
body contains "[teacher notes (2): open prompter]"

# Step 9: The student sees neither on the lesson page
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body not contains "{{note_one}}"
body not contains "open prompter"
body not contains "data-prompter-student"

# Step 10: The teacher's own dashboard offers [open prompter]
GET {{base_url}}/user/{{teacher_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "[open prompter]"
