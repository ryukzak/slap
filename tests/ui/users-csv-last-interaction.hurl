# The students CSV report ends with a "Last Interaction" column: when the
# student last submitted work or a teacher last checked it. Empty until the
# student has done either.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).

# Step 1: Sign in as teacher, then sign up a student

POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: LastSeen{{student_id}}

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Before any submission the column is present but empty

GET {{base_url}}/users/csv
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body matches "(?m)^ID,Name,.*,Bonus/Penalty,Last Interaction\\r?$"
body matches "(?m)^{{student_id}},LastSeen{{student_id}},.*,-?\\d+,\\r?$"

# Step 3: The student submits work

POST {{base_url}}/user/{{student_id}}/task/task1/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Last interaction submission

HTTP 303

# Step 4: The submission time now fills the column

GET {{base_url}}/users/csv
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body matches "(?m)^{{student_id}},LastSeen{{student_id}},.*,-?\\d+,\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}\\r?$"
