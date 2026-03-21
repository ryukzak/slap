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

1. Write an hurl test covering the broken behavior (place UI-specific tests in a separate file)
2. Run the test and confirm it **fails**
3. Fix the bug
4. Run the test again and confirm it **passes**

## Testing

```bash
make test           # unit tests
make test-hurl-ci   # integration tests (builds server, runs hurl, tears down)
make test-e2e-ci    # e2e tests (builds server, runs playwright, tears down)
```
