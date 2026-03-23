# Task Markdown Rendering Test
# Verifies that task record content with markdown syntax is rendered
# as HTML (e.g. **bold** becomes <strong>) and raw syntax is absent.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/task-markdown-rendering.hurl

# Step 1: Sign up student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: student_username=Markdown Student
variable: task_id=task1
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits a task record with markdown
POST {{base_url}}/user/{{student_id}}/task/{{task_id}}/journal
Cookie: user_data={{student_token}}
[FormParams]
content: **bold text** and _italic text_

HTTP 303

# Step 3: Task page renders markdown as HTML
GET {{base_url}}/user/{{student_id}}/task/{{task_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "<strong>bold text</strong>"
body contains "<em>italic text</em>"
body not contains "**bold text**"
body not contains "_italic text_"
