# slap

[![CI](https://github.com/ryukzak/slap/actions/workflows/ci.yml/badge.svg)](https://github.com/ryukzak/slap/actions/workflows/ci.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/ryukzak/slap)](https://hub.docker.com/r/ryukzak/slap)
[![Docker Image Version](https://img.shields.io/docker/v/ryukzak/slap?label=docker)](https://hub.docker.com/r/ryukzak/slap)

Student Lesson & Attempts Platform. A lightweight queue and task management system for university courses with multiple teachers.

## What it does

**Students** submit task solutions, register for lessons, and defend their work in a queued review session.

**Teachers** schedule lessons, review queued submissions one by one, leave feedback or scores, and track progress across all students.

**Teacher dashboard** (`/users`) provides a day-by-day activity timeline, a course-wide per-task activity heatmap (one row per task, one cell per week spanning the whole course), per-task statistics with visual bars, a students table with scores and statuses, and CSV export. The teacher directory (`/teachers`) adds per-teacher stats and a recent-scores log.

Under the hood: embedded BoltDB (no external database), JWT authentication, Markdown rendering, self-hosted static assets.

## Quick Start

### Docker (recommended)

```sh
docker compose up
```

The app will be available at http://localhost:8080.

### Local

```sh
make build
./slap
```

Or directly:

```sh
SLAP_JWT_SECRET=test go run main.go
```

## Configuration

### Config file

Default path: `conf/config.yaml` (override with `-config` flag or `SLAP_CONF` env var).

```yaml
name: "csa.2026.2"

teacher_ids:
  - "123"

default_lesson_description: |
  ## Agenda

  ## Notes

variants_file: assigned-vars.csv

report_token: "<output of: openssl rand -hex 32>"

tasks:
  - id: "task1"
    title: "Lab 1"
    description: |
      # Task description (Markdown)
    visible: true
    tags:
      - "approve"
```

- **name** — instance name shown in the page header (e.g. a course code such as `csa.2026.2`), so instances for different courses can be told apart; defaults to `slap`
- **teacher_ids** — users signing up with a matching ID get the teacher role
- **tasks** — task list available to all students
  - **visible** — when `true`, students see a short excerpt and tags of other students' submissions for this task on a shared lesson page (not just their own), so they can spot duplicate topics before the lesson; defaults to `false`. Full content stays restricted to the author and teachers.
  - **tags** — this task's allow-list of tag names shown on the lesson page. Only a teacher can tag a task record, by writing `#tag` (and remove one with `-#tag`) anywhere in a review — a student writing `#tag` in their own submission has no effect. The task page always shows every active tag, but the lesson page only shows tags listed here — so a tag can be used freely and "promoted" to the lesson page later just by adding it to this task's list. Scoped per task, so a tag meant for one task (e.g. a group tag on a presentation task) can't unexpectedly show up as "allowed" on another task's lesson-page rows.
- **variants_file** — optional `;`-separated CSV assigning each student a variant per task. The first row is `id;<task-id>;<task-id>;...`, every other row is `<student-id>;<variant>;...`; an empty cell means no variant for that task. A relative path is resolved against the config file's directory, and the file is read once at startup. The variant is shown on the student's task page and in the student's queue row details on the lesson page (to teachers and the student only). Every task column must name a configured task, and a student may appear only once.
- **report_token** — optional secret that lets a script fetch the students CSV report (`/users/csv`) by sending `Authorization: Bearer <token>` instead of signing in as a teacher, e.g. to [sync it into a Google Sheet](#syncing-the-report-to-google-sheets). Must be at least 32 characters; generate one with `openssl rand -hex 32`. Anyone holding it can read every student's scores, so keep it out of version control and rotate it by changing the value and restarting. When unset, the report is teacher-only.
- **default_lesson_description** — pre-filled text when creating a new lesson; once a teacher has created a lesson, their browser remembers the last 5 descriptions used, pre-fills the latest and offers all of them (plus this default) as presets
- **course_start** / **course_end** — optional; when both are set, they bound the shared time scale used by the activity timeline on a student's profile page (every student's timeline is then plotted on the same window, so positions are directly comparable across students) and by the teacher dashboard's course activity heatmap (its weekly columns then span this window instead of the earliest-to-latest record seen). When unset, both fall back to the earliest-to-latest task activity actually recorded.
- **token_duration_hours** — how many hours a signed-in session (JWT + cookie) stays valid before requiring sign-in again; defaults to `24`. Must be positive.
- **lesson_duration_minutes** — length of a lesson, used as the end time of the event in the lesson's `[add to calendar]` (`.ics`) file; defaults to `90`. Must be positive.
- **lesson_check_window_before_hours** / **lesson_check_window_after_hours** — how far before and after a lesson's scheduled start a check left from the *task page* still counts as that lesson's work, provided the acting teacher owns the lesson; default to `1` and `6`. Outside that window the check is treated as taken outside the lesson: the registration is dropped first, so the lesson neither keeps the student queued nor is credited with the check. Reviews submitted from the lesson page itself always belong to that lesson and ignore the window, so setting both to `0` effectively requires the lesson page for a lesson check. Must not be negative.

### Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SLAP_JWT_SECRET` | yes | — | Secret key for JWT token signing |
| `SLAP_PORT` | no | `8080` | HTTP port (also `-port` flag) |
| `SLAP_CONF` | no | `conf/config.yaml` | Config file path (also `-config` flag) |
| `SLAP_DB` | no | `tmp/slap.db` | BoltDB database file path |
| `SLAP_TZ` | no | `Europe/Moscow` | Primary timezone for date display |
| `SLAP_SECURE_COOKIES` | no | `false` | Set to `true` for HTTPS deployments |
| `SLAP_POSTHOG_KEY` | no | built-in | PostHog analytics API key |
| `SLAP_POSTHOG_HOST` | no | `https://eu.i.posthog.com` | PostHog host URL |

### Syncing the report to Google Sheets

With `report_token` set, a Google Apps Script bound to a spreadsheet can pull `/users/csv` on a schedule. The report has one row per student: ID, name, a score per task, the bonus/penalty total, and `Last Interaction` (the student's latest submission or teacher check, in `SLAP_TZ`). The script runs on Google's servers, so SLAP must be reachable from the internet.

1. Set `report_token` in the config and restart SLAP. Check it works:
   ```sh
   curl -H "Authorization: Bearer $TOKEN" https://slap.example.com/users/csv
   ```
2. Create a Google Sheet and add a tab named `Students`. The script overwrites this tab on every run, so keep your own notes and formulas on other tabs (look rows up by ID with `XLOOKUP`/`VLOOKUP`).
3. Open **Extensions → Apps Script** and paste:
   ```js
   function syncSlapReport() {
     const props = PropertiesService.getScriptProperties();
     const resp = UrlFetchApp.fetch(props.getProperty('SLAP_URL') + '/users/csv', {
       headers: { Authorization: 'Bearer ' + props.getProperty('SLAP_REPORT_TOKEN') },
       followRedirects: false,
       muteHttpExceptions: true,
     });
     if (resp.getResponseCode() !== 200) {
       throw new Error('SLAP returned ' + resp.getResponseCode());
     }
     const rows = Utilities.parseCsv(resp.getContentText());
     const sheet = SpreadsheetApp.getActive().getSheetByName('Students');
     sheet.clearContents();
     sheet.getRange(1, 1, rows.length, rows[0].length).setValues(rows);
     sheet.getRange(1, rows[0].length + 2).setValue('synced ' + new Date().toISOString());
   }
   ```
4. In **Project Settings → Script Properties**, add `SLAP_URL` (e.g. `https://slap.example.com`, no trailing slash) and `SLAP_REPORT_TOKEN`. The token stays out of the script code, so sharing the sheet doesn't share it, though sheet editors can still open the script settings.
5. Click **Run** once and grant the permissions Google asks for (the script reads and writes this spreadsheet and fetches external URLs). No Google Cloud project or service account is needed: the script runs as you.
6. In **Triggers** (the clock icon), add a trigger: function `syncSlapReport`, event source *Time-driven*, e.g. every hour. Failed runs are emailed to you.

## Documentation

- [GUIDE.md](GUIDE.md) — user guide for students and teachers (in Russian)
- [FEATURES.md](FEATURES.md) — use cases and requirements
- [CONTRIBUTE.md](CONTRIBUTE.md) — contribution guidelines

## Development

```sh
make format    # format Go and HTML templates
make lint      # run golangci-lint and djlint
make test      # run Go unit tests
```

### Integration Tests

Run integration tests (builds server, runs hurl, tears down):

```sh
make test-hurl-ci
```

## Project Structure

```
conf/           Configuration files (config.yaml)
src/
  analytics/    PostHog analytics + logging
  auth/         JWT authentication
  config/       YAML config loading
  handlers/     HTTP handlers
  storage/      BoltDB persistence
  util/         Template helpers
static/         CSS, JS, images
templates/      HTML templates (Go html/template)
tests/          Hurl integration tests
```
