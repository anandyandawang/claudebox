# Integration Tests for Sandbox Construction

## Overview

Add integration tests that verify Docker sandbox infrastructure is set up correctly: image builds, files are placed properly, git branch exists, and network policy works. No Claude Code execution. Tests run locally only (requires Docker Desktop with `docker sandbox`).

## Prerequisites

- Docker Desktop with `docker sandbox` support
- Local execution only (no CI requirement)

## Structural Changes

### Move existing unit tests into `tests/unit/`

Relocate all `tests/*.bats` files into `tests/unit/`. Update the Makefile accordingly.

### New directory: `tests/integration/`

Integration tests go here, run separately via `make test-integration`.

### Split test helpers

Refactor `tests/test_helper/common.bash` into three files:

- **`common.bash`** — shared setup: bats library loading, `SCRIPT_DIR` resolution (updated to `../..` since tests are now two levels deep)
- **`unit.bash`** — loads `common.bash`, provides mock infrastructure (`create_mock`, `create_mock_script`, mock `setup`/`teardown`)
- **`integration.bash`** — loads `common.bash`, provides real Docker sandbox setup/teardown, skip logic if `docker sandbox` is unavailable

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
│   └── cleanup.bats
├── test_helper/
│   ├── common.bash
│   ├── unit.bash
│   └── integration.bash
├── setup_test_deps.sh
└── bats/
```

### Makefile

```makefile
test: test-unit

test-unit: setup-test-deps
	./tests/bats/bin/bats tests/unit/*.bats

test-integration: setup-test-deps
	./tests/bats/bin/bats tests/integration/*.bats

test-all: test-unit test-integration
```

## Integration Test Helper (`integration.bash`)

- Loads `common.bash` for bats libraries and `SCRIPT_DIR`
- `setup_file`: builds the template image once, creates a sandbox with a unique name prefix (`claudebox-inttest-$$`)
- `teardown_file`: removes the sandbox
- Skips all tests with a clear message if `docker sandbox` is not available (detected by running `docker sandbox ls` and checking exit code)
- No mocks — all Docker commands are real

## Test Groups

### create.bats (~3 tests)

Tests that template images build and sandboxes can be created.

- Python template image builds successfully (`docker build` exits 0)
- JVM template image builds successfully
- Sandbox is created with correct name format matching `<workspace>-<template>-sandbox-<timestamp>`

### filesystem.bats (~4 tests)

Tests that files inside a created sandbox are laid out correctly. Uses `docker sandbox exec` to inspect the container.

- Repo files exist at `/home/agent/workspace` (check for known file like `claudebox`)
- Git branch named `sandbox/*` is checked out inside the sandbox
- Claude config symlinks exist (`/home/user/.claude.json`)
- Claude binary wrapper is installed at `/usr/local/bin/claude` and contains `cd /home/agent/workspace`

### network.bats (~3 tests)

Tests network policy enforcement for templates with `allowed-hosts.txt`.

- Disallowed host (`example.com`) is blocked (curl fails with timeout)
- Allowed host (`api.github.com`) is reachable (curl succeeds)
- A sandbox created from a template without `allowed-hosts.txt` has unrestricted network access (requires a temporary template or skipping if no such template exists)

### cleanup.bats (~2 tests)

Tests sandbox removal against real sandboxes.

- `claudebox rm <name>` removes a specific sandbox (verified by `docker sandbox ls`)
- `claudebox rm all` removes all sandboxes for a workspace (creates two, removes all, verifies none remain)

## Design Decisions

- **`setup_file`/`teardown_file` over `setup`/`teardown`**: sandbox creation is slow (~10-30s), so we create once per file and run multiple assertions against it rather than creating per-test
- **Unique name prefix with PID**: `claudebox-inttest-$$` prevents collisions with real sandboxes or parallel test runs
- **Skip, don't fail, without Docker**: integration tests should be opt-in; if Docker sandbox isn't available, tests skip gracefully
- **Python template as default**: most tests use the python template since it's smaller/faster to build; JVM build is tested once in `create.bats`
- **No credential tests**: credential refresh requires macOS Keychain state that's hard to control in tests; left for future work
