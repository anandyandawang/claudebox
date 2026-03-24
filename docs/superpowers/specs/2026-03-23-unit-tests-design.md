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

### Docker mock

`common.bash` creates a temp directory with a `docker` stub script and prepends it to `$PATH`. The stub:

- Logs every invocation (arguments) to `$MOCK_DOCKER_LOG` for assertion
- Returns canned output controlled by `$MOCK_DOCKER_OUTPUT` (default: empty)
- Returns exit code controlled by `$MOCK_DOCKER_EXIT_CODE` (default: 0)

Tests that need custom mock behavior override the stub per-test by writing a new script to the mock directory.

### Other mocked binaries

- `security` (macOS Keychain CLI) — mocked the same way for `refresh_credentials` tests
- `curl` — not mocked directly; `curl` calls in `create.sh` run inside `docker sandbox exec`, so the `docker` mock handles them. The mock inspects exec arguments: when the command contains `curl` and `example.com`, it returns exit code 1 (blocked); when it contains `curl` and `api.github.com`, it returns exit code 0 (allowed). Firewall-failure tests flip these.
- `date` — mocked to return a fixed timestamp so sandbox name assertions are deterministic

### SCRIPT_DIR

`common.bash` sets `SCRIPT_DIR` to the real repo root. Tests that need a fake template directory (e.g., to test missing Dockerfile) create one in `$BATS_TEST_TMPDIR` and override `SCRIPT_DIR` for that test.

### Docker mock output format

`docker sandbox ls` outputs a header line followed by data rows. Mock output for `resume` and `rm all` tests must include this header so that `awk 'NR>1 {print $1}'` and `grep` patterns work correctly.

### Stdin for interactive prompts

`resume.sh` uses `read -r` for confirmation/picker prompts. Tests provide stdin via pipe or `<<<` redirection, e.g., `echo "Y" | cmd_resume` or `run bash -c 'echo 1 | cmd_resume'`.

### Teardown

Every test file uses a bats `teardown` function to clean up the temp mock directory and any other temp files created during the test.

## What to Test

### Dispatcher (`claudebox.bats`)

- Shows usage and exits 1 when called with no args
- Routes `ls` to `commands/ls.sh`
- Routes `rm` to `commands/rm.sh`
- Routes `resume` to `commands/resume.sh`
- Routes unknown command to `commands/create.sh` (template mode), forwarding the template name as first arg
- Lists available templates in usage output

### Create (`create.bats`)

- Fails with error when template has no Dockerfile
- Parses workspace argument (defaults to pwd)
- Parses agent args after `--`
- Generates sandbox name from workspace basename, template, and fixed timestamp
- Calls `docker build` with correct image name and template dir
- Calls `docker sandbox create` with correct arguments
- Calls `docker sandbox exec` to symlink host config (.claude.json, settings.json, plugins)
- Calls `docker sandbox exec` to copy workspace and create session branch
- Applies network policy when `allowed-hosts.txt` exists (including parsing comments/blank lines)
- Firewall verification succeeds (blocked host fails, allowed host succeeds)
- Firewall verification fails — aborts when blocked host is reachable
- Skips network policy when no `allowed-hosts.txt`
- Calls setup_environment, refresh_credentials, wrap_claude_binary (verified via mock docker log)
- Calls `docker sandbox run` with `--dangerously-skip-permissions`
- Passes agent args through to `docker sandbox run`

### Remove (`rm.bats`)

- Shows usage when called with no args
- Removes a named sandbox that exists
- Prints error when named sandbox not found
- `docker sandbox rm` failure propagates error on single-sandbox removal
- Partial name match: `grep -q` matches substrings (e.g., "foo" matches "foobar") — test documents this behavior
- `rm all` removes only sandboxes matching current workspace name
- `rm all` prints message when no sandboxes found
- `rm all` reports count of attempted removals (note: count includes failures due to `|| true` before increment)
- `rm all` continues on individual removal failures (uses `|| true`)

### Resume (`resume.bats`)

- Errors when no sandboxes exist for workspace
- Errors on unknown arguments
- Auto-selects when exactly one sandbox exists (stdin: "Y")
- Exits cleanly (exit 0) when user declines single-sandbox confirmation (stdin: "n")
- Picker works when multiple sandboxes exist (stdin: selection number)
- Picker rejects invalid input and re-prompts
- Calls setup_environment, refresh_credentials, wrap_claude_binary on resume
- Passes agent args after `--` through to `docker sandbox run`

### Helpers (`helpers.bats`)

- `refresh_credentials`: extracts credentials from keychain and writes to sandbox
- `refresh_credentials`: warns when no credentials found
- `setup_environment`: truncates persistent env file
- `setup_environment`: exports GITHUB_USERNAME when set
- `setup_environment`: configures JVM proxy when HTTPS_PROXY is set
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
