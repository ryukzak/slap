# Header instance name Test
# The `name` config field (e.g. a course code) is rendered in the page header
# so several SLAP instances can be told apart. It must be visible both to
# anonymous visitors and inside authenticated pages.
#
# Prerequisites: server running with default config (name: "slap.dev",
#   teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   tests/ui/header-slap-name.hurl

# Step 1: Anonymous landing page shows the instance name in the header
GET {{base_url}}/
[Options]
variable: slap_name=slap.dev
variable: teacher_password=TeacherPass123!

HTTP 200
[Asserts]
body contains "slap-name"
body contains "{{slap_name}}"

# Step 2: Authenticated page keeps the name next to the page breadcrumb
POST {{base_url}}/signin
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/user/{{teacher_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "slap-name"
body contains "{{slap_name}}"

GET {{base_url}}/logout
HTTP 303
