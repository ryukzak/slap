# Score Rule: checked_after_task
#
# Covers the "Lab 2 before Labs 3-4" rule from the default config: a task
# checked after any listed task (or still unchecked once one is) gets the
# penalty. The rule is active while nothing is checked, applies once Lab 3 is
# checked before Lab 2, and stays applied after Lab 2 is checked late.
#
# Receives base_url, teacher_id, student_id from the runner.

# Student signs up.
POST {{base_url}}/signup
[Options]
variable: student_password=StudentPass123!
variable: teacher_password=TeacherPass123!
variable: student_username=TaskOrder Student
[FormParams]
id: {{student_id}}
password: {{student_password}}
username: {{student_username}}

HTTP 303
[Captures]
student_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Teacher signs in.
POST {{base_url}}/signin
[FormParams]
id: {{teacher_id}}
password: {{teacher_password}}

HTTP 303
[Captures]
teacher_token: cookie "user_data"

GET {{base_url}}/logout
HTTP 303

# Nothing checked yet: the rule is active and lists the tasks it compares with.
GET {{base_url}}/user/{{student_id}}/score-debug
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
jsonpath "$.rules[6].rule" == "Lab 2 before Labs 3-4"
jsonpath "$.rules[6].condition_kind" == "after_task"
jsonpath "$.rules[6].after_tasks" count == 2
jsonpath "$.rules[6].status" == "active"

# Lab 3 is checked while Lab 2 is not: the rule applies.
POST {{base_url}}/user/{{student_id}}/task/task3/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Lab 3 submission

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task3/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: 8 - Good

HTTP 303

GET {{base_url}}/user/{{student_id}}/score-debug
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
jsonpath "$.rules[6].status" == "applied"
jsonpath "$.rules[6].applied_effect" == -4

# Lab 2 checked afterwards is still out of order: the rule stays applied.
POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{student_token}}
[FormParams]
content: Lab 2 submission

HTTP 303

POST {{base_url}}/user/{{student_id}}/task/task2/journal
Cookie: user_data={{teacher_token}}
[FormParams]
role: review
content: 9 - Good

HTTP 303

GET {{base_url}}/user/{{student_id}}/score-debug
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
jsonpath "$.rules[6].status" == "applied"

# The profile shows the rule with the tasks it compares with.
GET {{base_url}}/user/{{student_id}}
Cookie: user_data={{student_token}}

HTTP 200
[Asserts]
body contains "Lab 2 before Labs 3-4"
xpath "string(//tr[td[contains(., 'Lab 2 before Labs 3-4')]]/td[3])" contains "after"
xpath "string(//tr[td[contains(., 'Lab 2 before Labs 3-4')]]/td[3])" contains "Lab 3"
xpath "string(//tr[td[contains(., 'Lab 2 before Labs 3-4')]]/td[3])" contains "Lab 4"
