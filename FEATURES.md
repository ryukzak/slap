# Use Case: student registration

1. Student opens the signup page and fills in:
   - ID (unique identifier, e.g. student number)
   - Username (display name)
   - Password
2. System checks that the ID is not already taken.
3. System validates password strength.
4. Account is created. Student is always assigned the student role.
5. System shows a token page with the generated JWT token and a "Sign in" link, or redirects to the dashboard directly, depending on configuration.
6. Student sees their dashboard with upcoming lessons, their tasks and submission statuses, and profile information.

# Use Case: teacher registration

Same as student registration, except:
- The teacher's ID must be listed in the server configuration beforehand.
- If the ID matches the config list, the account is assigned the teacher role in addition to student.
- A teacher is also a student and can submit tasks themselves.

Note: role assignment happens at signup and is re-evaluated from config on every sign-in. Changing the config list affects roles on the next login.

# Use Case: sign in

1. User opens the sign-in page and enters their ID and password.
2. System validates credentials.
3. System shows the token page with a "Sign in" link, or redirects to the dashboard directly, depending on configuration.
4. User sees their dashboard. The teacher dashboard includes a section for scheduling and managing lessons.

# Use Case: student submits a task

1. Student opens their profile page and selects a task.
2. Student reads the task description.
3. Student writes their solution or progress note in the text field and submits.
   - This creates a task record with status `submit`.
   - A student can add multiple records over time; each is appended to the task history.
4. Student can view the full history of their records on the task page.
   - Records with status `review` are teacher feedback entries.

# Use Case: teacher checks a student task (outside of a lesson)

1. Teacher opens a student's task page (via the student's profile).
2. Teacher reads the full history of the student's records.
3. Teacher writes feedback in the review field and submits.
   - This creates a new record with status `review` authored by the teacher.
   - If the student's latest record had status `register`, it is automatically transitioned to `reviewed`.
4. Student sees the teacher feedback on their task page.

# Use Case: teacher prepares queue for the lesson

1. Teacher schedules a lesson with a date and time.
2. Teacher adds a short description of the lesson to set student expectations.
3. Student registers for the lesson before it starts — registration is blocked once the lesson begins.
   - The student must have an existing `submit` record for the task they want to register.
   - Only one registration per task per lesson at a time.
   - If the student updates their task after registering, the registration is automatically revoked.
4. At lesson start, the teacher opens the lesson queue and reviews submitted tasks one by one:
   - Open the first submitted task.
   - Read the full history of the task (student and teacher entries).
   - Submit feedback via the review form — the student will see it on their task page.
   - Move to the next task from another student.
5. Teacher can see which tasks have been reviewed and which are still pending.

# General requirements

1. Teacher can see all profiles, tasks, and their statuses.
2. Student can see only their own task submissions, and the lesson list and schedules.
3. All multiline text fields should support markdown formatting with rendered output.
