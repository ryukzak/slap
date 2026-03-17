# Contributing

## Before you start

- Open an issue or discuss the change before writing code.
- Keep PRs small and focused — one feature or fix per PR.
- A PR that does too many things at once will be asked to split.

## Requirements for every PR

- All tests must pass: `make test && make test-hurl-ci`
- Code must be formatted and lint-clean: `make format && make lint`
- If you changed UI — attach screenshots to the PR description.

## How to submit

1. Fork the repo and create a branch from `master`.
2. Make your change.
3. Run the full test suite locally (see above).
4. Open a PR with a clear description of what and why.
5. Wait for review — do not merge without approval.

## Style

- Follow existing code patterns; do not refactor unrelated code.
- Template changes: run `make format` (djlint) before committing.
- Commit messages: short imperative subject line, e.g. `Add deadline extension for lessons`.

## What gets rejected

- PRs without passing tests.
- PRs without screenshots when UI changed.
- Large PRs mixing unrelated changes.
- Refactoring mixed with new features — always separate into different PRs.
- Breaking changes without prior discussion.
