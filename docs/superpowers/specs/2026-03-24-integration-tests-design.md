# Integration Tests for Sandbox Construction

## Overview

Add integration tests that verify Docker sandbox infrastructure is set up correctly: image builds, files are placed properly, git branch exists, and network policy works. No Claude Code execution. Tests run locally only (requires Docker Desktop with `docker sandbox`).

## Prerequisites

- Docker Desktop with `docker sandbox` support
- Local execution only (no CI requirement)

## Structural Changes

### Move existing unit tests into `tests/unit/`

Relocate all `tests/*.bats` files into `tests/unit/`. Refactor existing tests to load `unit.bash` instead of directly loading bats libraries or defining their own mock helpers. Remove duplicated `create_mock`, `create_mock_script`, `setup`, and `teardown` definitions from individual test files — these now live in `unit.bash`.

### New directory: `tests/integration/`

Integration tests go here, run separately via `make test-integration`.

### Split test helpers

Refactor `tests/test_helper/common.bash` into three files:

- **`common.bash`** — shared setup: bats library loading via absolute paths (`load "${SCRIPT_DIR}/tests/test_helper/bats-support/load"`), `SCRIPT_DIR` resolution (updated to `../..` since tests are now two levels deep under `tests/unit/` or `tests/integration/`)
- **`unit.bash`** — loads `common.bash`, provides mock infrastructure (`create_mock`, `create_mock_script`, mock `setup`/`teardown`)
- **`integration.bash`** — loads `common.bash`, provides real Docker sandbox setup/teardown, skip logic if `docker sandbox` is unavailable

Since tests move from `tests/` to `tests/unit/` (or `tests/integration/`), all bats `load` paths must use absolute paths derived from `SCRIPT_DIR` rather than relative paths. Each test file loads either `unit.bash` or `integration.bash`, which in turn loads `common.bash`.

### File structure

```
tests/
├── unit/
│   ├── claudebox.bats
│   ├── create.bats
│   ├── helpers.bats
│   ├── ls.bats
│   ├── resume.bats
│   └── rm.bats
├── integration/
│   ├── create.bats
│   ├── filesystem.bats
│   ├── network.bats
│   └── zz_cleanup.bats
├── test_helper/
│   ├── common.bash
│   ├── unit.bash
│   └── integration.bash
├── setup_test_deps.sh
└── bats/
```

### Makefile

Retain existing `setup-test-deps` and `clean` targets unchanged. Update test targets:

```makefile
.PHONY: test test-unit test-integration test-all setup-test-deps clean

test: test-unit

test-unit: setup-test-deps
	./tests/bats/bin/bats tests/unit/*.bats

test-integration: setup-test-deps
	./tests/bats/bin/bats tests/integration/*.bats

test-all: test-unit test-integration

setup-test-deps:
	@./tests/setup_test_deps.sh

clean:
	rm -rf tests/bats tests/test_helper/bats-support tests/test_helper/bats-assert
```

## Integration Test Helper (`integration.bash`)

- Loads `common.bash` for bats libraries and `SCRIPT_DIR`
- Skips all tests with a clear message if `docker sandbox` is not available (detected by running `docker sandbox ls` and checking exit code)
- No mocks — all Docker commands are real

**Sandbox creation approach:** Integration tests call `cmd_create` (by sourcing `src/commands/create.sh`) with a temporary workspace directory whose basename is `claudebox-inttest-<pid>`. This produces sandbox names in the standard format (`claudebox-inttest-<pid>-<template>-sandbox-<timestamp>`), making them identifiable and safe to clean up.

**Sandbox sharing:** `filesystem.bats` and `network.bats` each create their own sandbox in `setup_file` and tear it down in `teardown_file`. `create.bats` creates/destroys sandboxes within individual tests. `zz_cleanup.bats` creates its own sandboxes for removal testing — it uses a distinct workspace basename (`claudebox-inttest-cleanup-<pid>`) to avoid interfering with other test files.

## Test Groups

### create.bats (~3 tests)

Tests that template images build and sandboxes can be created. Each test manages its own sandbox lifecycle.

- Python template image builds successfully (`docker build` exits 0)
- JVM template image builds successfully
- Sandbox is created with correct name format matching `<workspace>-<template>-sandbox-<timestamp>`

### filesystem.bats (~4 tests)

Tests that files inside a created sandbox are laid out correctly. Uses `docker sandbox exec` to inspect the container. Shares a single python-template sandbox across all tests via `setup_file`/`teardown_file`.

- Repo files exist at `/home/agent/workspace` (check for known file like `claudebox`)
- Git branch matching `sandbox-*` pattern is checked out inside the sandbox
- Claude config symlinks exist (`/home/agent/.claude.json`)
- Claude binary wrapper is installed at `/usr/local/bin/claude` and contains `cd /home/agent/workspace`

### network.bats (~3 tests)

Tests network policy enforcement for templates with `allowed-hosts.txt`. Shares a single python-template sandbox via `setup_file`/`teardown_file`.

- Disallowed host (`example.com`) is blocked (curl fails with timeout)
- Allowed host (`api.github.com`) is reachable (curl succeeds)
- Unrestricted access test: creates a temporary template directory containing only a Dockerfile (copied from python template, no `allowed-hosts.txt`), builds a sandbox from it, verifies `example.com` is reachable, then tears down the temporary sandbox and template

### zz_cleanup.bats (~2 tests)

Tests sandbox removal against real sandboxes. Named with `zz_` prefix to ensure it runs last in alphabetical glob order, after other test files have cleaned up their own sandboxes. Uses a distinct workspace basename (`claudebox-inttest-cleanup-<pid>`) so `rm all` only affects its own sandboxes.

- `claudebox rm <name>` removes a specific sandbox (verified by `docker sandbox ls`)
- `claudebox rm all` removes all sandboxes for a workspace (creates two, removes all, verifies none remain)

## Design Decisions

- **`setup_file`/`teardown_file` over `setup`/`teardown`**: sandbox creation is slow (~10-30s), so we create once per file and run multiple assertions against it rather than creating per-test
- **Standard sandbox naming via `cmd_create`**: tests use the real create flow with a controlled workspace basename, so sandbox names follow the production format and the full creation logic is exercised
- **Unique workspace basenames per test file**: prevents `rm all` in cleanup tests from destroying sandboxes owned by other test files
- **`zz_` prefix for cleanup tests**: ensures cleanup runs last in bats glob order, avoiding interference with other test files
- **Skip, don't fail, without Docker**: integration tests should be opt-in; if Docker sandbox isn't available, tests skip gracefully
- **Python template as default**: most tests use the python template since it's smaller/faster to build; JVM build is tested once in `create.bats`
- **No credential tests**: credential refresh requires macOS Keychain state that's hard to control in tests; left for future work
