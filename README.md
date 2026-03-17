# slap

[![CI](https://github.com/ryukzak/slap/actions/workflows/ci.yml/badge.svg)](https://github.com/ryukzak/slap/actions/workflows/ci.yml)

Student Lesson & Attempts Platform. A lightweight queue and task management system for university courses with multiple teachers.

## Features

- Registration with ID, name, and group
- Config-based teacher roles via GUID list in `conf/config.yaml`
- Task journals with lesson scheduling
- Markdown rendering for task descriptions
- Embedded BoltDB storage (no external database required)
- JWT-based authentication

## Quick Start

### Docker (recommended)

```sh
docker compose up
```

The app will be available at http://localhost:8080.

To customize the config, edit `conf/config.yaml` — changes are picked up on restart.

### Local

```sh
make build
./slap
```

Or directly:

```sh
go run main.go
```

Flags:
- `-port` — HTTP port (default: `8080`)
- `-config` — config file path (default: `conf/config.yaml`)

## Configuration

Teacher accounts are defined by ID in `conf/config.yaml`:

```yaml
teacher_ids:
  - "teacher-guid-1"
  - "teacher-guid-2"
```

Users signing up with a matching ID get the teacher role. All other signups create student accounts.

## Development

```sh
make format    # format Go and HTML templates
make lint      # run golangci-lint and djlint
make test      # run Go unit tests
```

### Integration Tests

Start the server, then run [hurl](https://hurl.dev) tests:

```sh
./slap &
hurl --test --variable "test_user_id=testuser_$(date +%s)" tests/auth-flow.hurl
```

## Project Structure

```
conf/           Configuration files (config.yaml)
src/
  auth/         JWT authentication
  config/       YAML config loading
  handlers/     HTTP handlers
  storage/      BoltDB persistence
  util/         Template helpers
static/         CSS, JS, images
templates/      HTML templates (Go html/template)
tests/          Hurl integration tests
```
