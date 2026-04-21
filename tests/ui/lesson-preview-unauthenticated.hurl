# Lesson Preview for Unauthenticated Users
# Tests: unauthenticated user sees lesson info and sign-in link,
#        authenticated user sees the full lesson page.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).

# Step 1: Sign in as teacher and create a lesson
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
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
date: 2099-06-15
time: 10:00
description: Preview test lesson description

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

GET {{base_url}}/logout
HTTP 303

# Step 2: Unauthenticated user visits lesson page — sees preview
GET {{base_url}}/lesson/{{lesson_id}}

HTTP 200
[Asserts]
body contains "Preview test lesson description"
body contains "2099"
body contains "[sign in]"
body not contains "[delete]"
body not contains "[logout]"
body not contains "// registered tasks"
body not contains "// edit description"
body contains "og:title"
body contains "og:description"
body contains "Preview test lesson description"

# Step 3: Authenticated user sees full lesson page
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Preview test lesson description"
body contains "[logout]"
body contains "// registered tasks"
