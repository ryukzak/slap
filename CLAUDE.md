# SLAP Project

## Setup

After cloning, install the pre-commit hook:

```bash
make install-hooks
```

Requires `djlint` for template formatting: `pip install djlint==1.36.4`

## Before Committing

The pre-commit hook runs `make format-check lint` automatically. To fix issues:

```bash
make format   # formats Go files (gofmt) and HTML templates (djlint)
make lint     # runs golangci-lint and djlint --lint
```

## Build

```bash
make build    # builds with version info injected (git describe + timestamp)
make run      # build and run locally
```

## UI Conventions

### Buttons

Three roles, all use lowercase bracket notation `[label]`:

- **Primary** — `bg-blue-800 hover:bg-blue-700 text-xs px-3 py-1 rounded` — main actions (`[add]`, `[sign in]`)
- **Ghost** — `text-blue-400 hover:text-blue-300 text-xs` — navigation and secondary actions (`[back]`, `[register]`, `[view task]`)
- **Danger** — `text-red-400 hover:text-red-300 text-xs` — destructive actions (`[delete]`, `[revoke]`)

### Section Headers

Inside `<section>` content, use a `<div>` with `text-xs text-gray-500 mb-2` and all-lowercase `//` prefix:

```html
<div class="text-xs text-gray-500 mb-2">
  // section name
</div>
```

### Page Header Breadcrumb

The top bar follows: `SLAP · // page-name` on the left, nav links on the right.

- `{{define "header"}}` — adds `· // page-name` (e.g. `// lessons`, `// task: title`)
- `{{define "footer_nav"}}` — adds nav links (`[back]`, `[logout]`, `[delete]`)

All nav links use ghost button style. No IDs or raw data in the header.

## Fixing UI Bugs

For UI bug fixes (except purely visual/style changes), follow TDD with hurl tests:

1. Write an hurl test covering the broken behavior (place in `tests/ui/`)
2. Run the test and confirm it **fails**
3. Fix the bug
4. Run the test again and confirm it **passes**

### Writing UI hurl tests

Place UI tests in `tests/ui/`, named after the use case (e.g. `user-list-student-row.hurl`).

**Variables** — tests receive `base_url`, `teacher_id` from the runner. Define passwords, usernames, and other constants via `[Options] variable:` on the first request. Use `student_id` (passed from runner) to keep users unique across tests.

**Auth pattern** — the teacher (id `123`) is signed up by earlier tests; use `POST /signin` to get its token. For students, use `POST /signup`. Always `GET /logout` between user switches to clear the cookie jar, then pass tokens explicitly via `Cookie: user_data={{token}}`.

**Assertions** — use `body contains` / `body not contains` to check rendered HTML: links (`/user/{{student_id}}/task/task1`), text content (`@Username`, `Queued`), and data (`(q:1)`, `[0/1]`).

**Wiring** — after creating a test file, register it in both:
- `tests/run-hurl.sh` — add a `run_test` call with a unique `student_id` suffix (`${TIMESTAMP}<N>`)
- `Makefile` `test-hurl` target — add a matching `hurl --test` entry

## Before Creating a PR

Check that `README.md`, `GUIDE.md`, and `FEATURES.md` reflect any new or changed functionality. If a PR adds features, changes UI, or modifies behavior, ask the user whether the docs need updating before opening the PR.

## Testing

```bash
make test           # unit tests
make test-hurl-ci   # integration tests (builds server, runs hurl, tears down)
make test-e2e-ci    # e2e tests (builds server, runs playwright, tears down)
```
