# Unit Tests for claudebox

## Overview

Add unit tests to claudebox using bats-core. Tests mock the `docker` CLI via PATH-based stubs so they run fast without Docker.

## Prerequisites

- bats-core, bats-assert, bats-support (Homebrew)

## Directory Structure

```
tests/
  test_helper/
    common.bash      # shared setup: mock docker, set SCRIPT_DIR, source helpers
  create.bats        # commands/create.sh
  rm.bats            # commands/rm.sh
  resume.bats        # commands/resume.sh
  ls.bats            # commands/ls.sh
  helpers.bats       # lib/helpers.sh
  claudebox.bats     # top-level dispatcher
```

## Mock Strategy

`common.bash` creates a temp directory with a `docker` stub script and prepends it to `$PATH`. The stub:

- Logs every invocation (arguments) to `$MOCK_DOCKER_LOG` for assertion
- Returns canned output controlled by `$MOCK_DOCKER_OUTPUT` (default: empty)
- Returns exit code controlled by `$MOCK_DOCKER_EXIT_CODE` (default: 0)

For `helpers.sh` tests, `security` (macOS Keychain CLI) is also mocked the same way.

Tests that need custom mock behavior override the stub per-test by writing a new script to the mock directory.

## What to Test

### Dispatcher (`claudebox.bats`)

- Shows usage and exits 1 when called with no args
- Routes `ls` to `commands/ls.sh`
- Routes `rm` to `commands/rm.sh`
- Routes `resume` to `commands/resume.sh`
- Routes unknown command to `commands/create.sh` (template mode)
- Lists available templates in usage output

### Create (`create.bats`)

- Fails with error when template has no Dockerfile
- Parses workspace argument (defaults to pwd)
- Parses agent args after `--`
- Generates sandbox name from workspace basename, template, and timestamp
- Calls `docker build` with correct image name and template dir
- Calls `docker sandbox create` with correct arguments
- Applies network policy when `allowed-hosts.txt` exists
- Skips network policy when no `allowed-hosts.txt`
- Calls `docker sandbox run` with `--dangerously-skip-permissions`
- Passes agent args through to `docker sandbox run`

### Remove (`rm.bats`)

- Shows usage when called with no args
- Removes a named sandbox that exists
- Prints error when named sandbox not found
- `rm all` removes only sandboxes matching current workspace name
- `rm all` prints message when no sandboxes found
- `rm all` reports count of removed sandboxes

### Resume (`resume.bats`)

- Errors when no sandboxes exist for workspace
- Errors on unknown arguments
- Auto-selects when exactly one sandbox exists (simulated confirmation)
- Calls setup_environment, refresh_credentials, wrap_claude_binary on resume
- Passes agent args after `--` through to `docker sandbox run`

### Helpers (`helpers.bats`)

- `refresh_credentials`: extracts credentials from keychain and writes to sandbox
- `refresh_credentials`: warns when no credentials found
- `setup_environment`: truncates persistent env file
- `setup_environment`: exports GITHUB_USERNAME when set
- `wrap_claude_binary`: creates wrapper script
- `wrap_claude_binary`: does not overwrite existing claude-real (idempotency)

### Ls (`ls.bats`)

- Delegates to `docker sandbox ls`

## Running Tests

```bash
# Install dependencies
brew install bats-core bats-assert bats-support

# Run all tests
bats tests/

# Run a single test file
bats tests/rm.bats
```
