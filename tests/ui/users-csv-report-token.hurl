# The students CSV report (/users/csv) can be fetched without a teacher
# session by presenting the config's report_token as a Bearer token, so a
# script (e.g. Google Apps Script) can sync it into a spreadsheet.

#
# Prerequisites: server running against a fresh DB with conf/config.yaml
#   (report_token: dev-report-token-0123456789abcdef).

# Step 1: Sign up a student so the report has a row to show

POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: report_token=dev-report-token-0123456789abcdef
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: CsvToken{{student_id}}

HTTP 303

GET {{base_url}}/logout
HTTP 303

# Step 2: The report token alone returns the CSV with the student

GET {{base_url}}/users/csv
Authorization: Bearer {{report_token}}

HTTP 200
[Asserts]
header "Content-Type" contains "text/csv"
body contains "{{student_id}},CsvToken{{student_id}}"

# Step 3: A wrong token is refused and leaks no data

GET {{base_url}}/users/csv
Authorization: Bearer {{report_token}}-wrong

HTTP *
[Asserts]
status != 200
body not contains "CsvToken{{student_id}}"

# Step 4: No credentials at all is refused too

GET {{base_url}}/users/csv

HTTP *
[Asserts]
status != 200
body not contains "CsvToken{{student_id}}"
