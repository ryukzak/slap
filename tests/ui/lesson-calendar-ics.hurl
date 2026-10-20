# Lesson Calendar (.ics) Test
# The lesson page offers an [add to calendar] button linking to an iCalendar
# file for the lesson. Times are emitted in UTC ("Z" suffix) so every calendar
# shows the lesson in the viewer's own timezone. The lesson is created at
# 18:00 in the server's primary timezone (SLAP_TZ, default Europe/Moscow,
# UTC+3), so it must start at 15:00Z and last the default 90 minutes.
#
# Prerequisites: server running against a fresh DB with default config
#   (teacher_ids: ["123"]) and default SLAP_TZ.
# Run: hurl --test \
#   --variable base_url=http://localhost:8080 \
#   --variable teacher_id=123 \
#   --variable student_id=$(date +%s)43 \
#   tests/ui/lesson-calendar-ics.hurl

# Step 1: Sign in as teacher, create a lesson
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
date: 2099-12-31
time: 18:00
description: Calendar test lesson, with; special chars

HTTP 200
[Captures]
lesson_id: url regex "/lesson/([^/?#]+)"

# Step 2: The lesson page links to the calendar file
GET {{base_url}}/lesson/{{lesson_id}}
Cookie: user_data={{teacher_token}}

HTTP 200
[Asserts]
body contains "/lesson/{{lesson_id}}/calendar.ics"
body contains "[add to calendar]"

GET {{base_url}}/logout
HTTP 303

# Step 3: The unauthenticated preview links to it too
GET {{base_url}}/lesson/{{lesson_id}}

HTTP 200
[Asserts]
body contains "/lesson/{{lesson_id}}/calendar.ics"

# Step 4: The calendar file is a valid single-event iCalendar in UTC
GET {{base_url}}/lesson/{{lesson_id}}/calendar.ics

HTTP 200
[Asserts]
header "Content-Type" contains "text/calendar"
body contains "BEGIN:VCALENDAR\r\n"
body contains "BEGIN:VEVENT\r\n"
body contains "UID:{{lesson_id}}@"
body contains "DTSTART:20991231T150000Z\r\n"
body contains "DTEND:20991231T163000Z\r\n"
body contains "/lesson/{{lesson_id}}"
body contains "Calendar test lesson\\, with\\; special chars"
body contains "END:VCALENDAR\r\n"

# Step 5: Unknown lesson returns 404
GET {{base_url}}/lesson/no-such-lesson/calendar.ics

HTTP 404
