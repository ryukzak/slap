# Tag Detail Reviewed Excerpt Test
# The tag detail page (/tags/{tag}) excerpt used to show the student's
# latest own message unconditionally. That breaks when a student resubmits
# placeholder content (e.g. "just for registration") purely to satisfy the
# submit/revoke precondition for re-registering into a lesson: the real
# topic a teacher actually reviewed and tagged gets replaced by the
# placeholder. The excerpt now shows the submission the most recent teacher
# review responded to, ignoring any later placeholder resubmission.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]).
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)40 \
#   tests/ui/tag-detail-reviewed-excerpt.hurl

# Step 1: Sign in as teacher
POST {{base_url}}/signin
[Options]
variable: teacher_password=TeacherPass123!
variable: student_password=StudentPass123!
variable: student_username=ExcerptStudent
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Step 2: Student submits the real topic
POST {{base_url}}/signup
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Real topic implement quicksort

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 3: Teacher reviews and tags it
POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{teacher_token}}
[FormParams]
content: Looks good \#excerpttag

HTTP 303

# Step 4: Student resubmits placeholder content just to satisfy the
# re-registration precondition -- the real topic must stay the excerpt
POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: just for registration

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 5: Tag detail page still shows the reviewed topic, not the placeholder
GET {{base_url}}/tags/excerpttag
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "Real topic implement quicksort"
body not contains "just for registration"

GET {{base_url}}/logout
HTTP 303
