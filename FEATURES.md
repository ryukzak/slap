# Use Case: student registration

1. Student opens the signup page and fills in:
   - ID (unique identifier, e.g. student number)
   - Username (display name)
   - Password
2. System checks that the ID is not already taken.
3. System validates password strength.
4. Account is created. Student is always assigned the student role.
5. Student is redirected to their dashboard.

# Use Case: teacher registration

Same as student registration, except:
- The teacher's ID must be listed in the server configuration beforehand.
- If the ID matches the config list, the account is assigned the teacher role in addition to student.
- A teacher is also a student and can submit tasks themselves.

Note: role assignment happens at signup and is re-evaluated from config on every sign-in. Changing the config list affects roles on the next login.

# Use Case: sign in

1. User opens the sign-in page and enters their ID and password.
2. System validates credentials.
3. User is redirected to their dashboard. The teacher dashboard includes a section for scheduling and managing lessons.

# Use Case: student submits a task

1. Student opens their profile page and selects a task.
2. Student reads the task description.
3. Student writes their solution or progress note in the text field and submits.
   - This creates a task record with status `submit`.
   - A student can add multiple records over time; each is appended to the task history.
4. Student can view the full history of their records on the task page.
   - Records with status `review` are teacher feedback entries, shown inline below the submission they belong to.
   - A compact summary of record counts by status is shown next to the task status (e.g. `p:2 f:1 c:1`).

# Use Case: teacher checks a student task (outside of a lesson)

1. Teacher opens a student's task page (via the student's profile).
2. Teacher reads the full history of the student's records.
3. Teacher writes feedback in the review field and submits.
   - This creates a new record with status `review` authored by the teacher.
   - If the student's latest record had status `register`, it is automatically transitioned to `reviewed`.
   - If the review text starts with a number, it is extracted as the score for that task.
4. Student sees the teacher feedback on their task page, shown inline below their submission.
5. Teacher can submit multiple feedback entries; all are shown concatenated with a separator.

# Use Case: teacher prepares queue for the lesson

1. Teacher schedules a lesson with a date and time.
2. Teacher adds a short description of the lesson to set student expectations.
3. Student registers for the lesson before it starts — registration is blocked once the lesson begins.
   - The student must have an existing `submit` record for the task they want to register.
   - Only one registration per task per lesson at a time.
   - If the student updates their task after registering, the registration is automatically revoked.
4. Teacher can extend the registration deadline beyond the lesson start time using the
   "extend registration" form on the lesson page. This allows late registrations after the
   lesson has already started.
5. At lesson start, the teacher opens the lesson queue and reviews submitted tasks one by one:
   - Open the first submitted task.
   - Read the full history of the task (student and teacher entries).
   - Submit feedback via the review form — the student will see it on their task page.
   - Move to the next task from another student.
6. Teacher can see which tasks have been reviewed and which are still pending.
7. Teacher can bulk-revoke all queued registrations via `[revoke all]`.
8. Teacher can edit the lesson description after creation.
9. Teacher can toggle `[show history]` to see previously revoked registration attempts.
10. Teacher can expand or collapse all task records with `[open all]`.

# Use Case: teacher views student dashboard

1. Teacher navigates to `/users` (the students page).
2. **Activity timeline** — shows a 3-week window (2 weeks past, 1 week future) with:
   - Past days: count of teacher reviews performed that day.
   - Future days: scheduled lessons with registered/reviewed counts.
3. **Task statistics** — per-task aggregate bars showing status distribution across all students
   (Pending, Queued, Feedback, Checked, Dropped, no submission).
4. **Students table** — all registered users with per-task columns. Each cell shows:
   - Score (if the teacher started a review with a number) and status badge.
   - Compact status summary (e.g. `p:2 q:1 c:1`).
5. Teacher can download the table as CSV via `[download csv]`.

# Use Case: lesson preview for unauthenticated users

1. An unauthenticated user opens a lesson page link (e.g. shared via Telegram).
2. The system shows a public preview with: teacher name, date/time, enrolled count, registration status, and the lesson description.
3. A `[sign in]` link is shown so the user can authenticate and register.
4. Telegram (and other messengers) can generate a link preview from the page title and content.
5. No task records, edit controls, or user-specific data are exposed.

# Use Case: task record visibility on lesson page

1. When a student views a lesson page, only their own task records are shown.
2. When a teacher views a lesson page, all task records from all students are shown.

# General requirements

1. Teacher can see all profiles, tasks, and their statuses.
2. Student can see only their own task submissions, and the lesson list and schedules.
3. On lesson pages, students only see their own records; teachers see all records.
4. All multiline text fields should support markdown formatting with rendered output.
5. Teacher has access to a users table (`/users`) showing all registered students with
   per-task columns. Each cell shows the score (if the teacher started the review with a
   number) and a status badge.
