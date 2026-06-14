# UI Test: Score Rules Display
# Verifies the score rules table renders correctly on student profile
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"], score_rules configured).
# Run: hurl --test \
#   --variable student_id=$(date +%s) \
#   --variable teacher_id=123 \
#   tests/ui/score-rules-student-visibility.hurl

# Sign up student
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: Score Rules UI Student

HTTP 303
[Captures]
student_token: cookie "user_data"

# Student profile should show score rules section
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "// score rules"
body contains "Bonus/Penalty"
body contains "total effect:"