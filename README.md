# slap

[![CI](https://github.com/ryukzak/slap/actions/workflows/ci.yml/badge.svg)](https://github.com/ryukzak/slap/actions/workflows/ci.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/ryukzak/slap)](https://hub.docker.com/r/ryukzak/slap)
[![Docker Image Version](https://img.shields.io/docker/v/ryukzak/slap?label=docker)](https://hub.docker.com/r/ryukzak/slap)

Student Lesson & Attempts Platform. A lightweight queue and task management system for university courses with multiple teachers.

## What it does

**Students** submit task solutions, register for lessons, and defend their work in a queued review session.

**Teachers** schedule lessons, review queued submissions one by one, leave feedback or scores, and track progress across all students.

**Teacher dashboard** (`/users`) provides an activity timeline, per-task statistics with visual bars, a students table with scores and statuses, and CSV export.

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
teacher_ids:
  - "123"

default_lesson_description: |
  ## Agenda

  ## Notes

tasks:
  - id: "task1"
    title: "Lab 1"
    description: |
      # Task description (Markdown)
```

- **teacher_ids** — users signing up with a matching ID get the teacher role
- **tasks** — task list available to all students
- **default_lesson_description** — pre-filled text when creating a new lesson

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
