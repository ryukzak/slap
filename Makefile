.PHONY: all build test test-unit test-hurl test-hurl-ci run clean format format-go format-templates format-check format-check-go format-check-templates lint lint-go lint-templates install-hooks

GOCMD=go
GOTEST=$(GOCMD) test
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get

MAIN_PACKAGE=.
PROJECT_NAME=slap
BINARY_NAME=$(PROJECT_NAME)

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION)-$(BUILD_TIME)"

all: format lint test

build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)

test: test-unit

test-unit:
	$(GOTEST) -v ./src/...

# Run Hurl integration tests against a running server.
# Usage: make test-hurl [SERVER=http://localhost:8080] [TEACHER_ID=123]
SERVER ?= http://localhost:8080
TEACHER_ID ?= 123
HURL_VARS = --variable base_url=$(SERVER) --variable teacher_id=$(TEACHER_ID)

test-hurl:
	hurl --test $(HURL_VARS) \
		--variable student_id=student_$$(date +%s)0 \
		tests/task-submit-flow.hurl
	hurl --test $(HURL_VARS) \
		--variable student_id=student_$$(date +%s)1 \
		tests/teacher-review-flow.hurl
	hurl --test $(HURL_VARS) \
		--variable student_id=student_$$(date +%s)2 \
		tests/lesson-queue-flow.hurl
	hurl --test $(HURL_VARS) \
		--variable student_id=student_$$(date +%s)3 \
		tests/lesson-registration-rules.hurl
	hurl --test $(HURL_VARS) \
		--variable student_a_id=student_a_$$(date +%s)4 \
		--variable student_b_id=student_b_$$(date +%s)5 \
		tests/access-control.hurl
	hurl --test $(HURL_VARS) \
		--variable student_id=student_$$(date +%s)10 \
		tests/ui/user-list-student-row.hurl
	hurl --test $(HURL_VARS) \
		--variable student_id=student_$$(date +%s)11 \
		tests/ui/task-registered-lesson-info.hurl
	hurl --test $(HURL_VARS) \
		--variable student_id=student_$$(date +%s)12 \
		tests/ui/dashboard-role-sections.hurl
	hurl --test $(HURL_VARS) \
		--variable student_id=student_$$(date +%s)13 \
		tests/ui/task-markdown-rendering.hurl

# Build, start server with a temp DB, run Hurl tests, stop server.
# Usage: make test-hurl-ci [TEST_PORT=18080] [TEACHER_ID=123]
TEST_PORT ?= 18080

test-hurl-ci:
	./tests/run-hurl.sh $(TEST_PORT) $(TEACHER_ID)

run:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)
	./$(BINARY_NAME)

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

format: format-go format-templates

format-go:
	gofmt -w .

format-templates:
	djlint templates --reformat

format-check: format-check-go format-check-templates

format-check-go:
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "Go files need formatting. Run 'make format-go'"; \
		gofmt -d .; \
		exit 1; \
	fi

format-check-templates:
	djlint templates --check

lint: lint-go lint-templates

lint-go:
	golangci-lint run

lint-templates:
	djlint templates --lint

install-hooks:
	@printf '#!/bin/sh\nmake format-check lint-templates\n' > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "pre-commit hook installed"
