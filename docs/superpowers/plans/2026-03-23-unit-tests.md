# claudebox Unit Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add comprehensive unit tests for all claudebox commands using bats-core with PATH-based docker mocking.

**Architecture:** Each source file gets a corresponding `.bats` test file. A shared `test_helper/common.bash` sets up a mock `docker` binary (and other external commands) via PATH prepending, so tests run fast without Docker. Tests source command files directly and call their exported functions.

**Tech Stack:** bats-core, bats-assert, bats-support (Homebrew)

**Spec:** `docs/superpowers/specs/2026-03-23-unit-tests-design.md`

---

## File Structure

```
tests/
  test_helper/
    common.bash      # shared setup/teardown, mock creation helpers
  helpers.bats       # tests for lib/helpers.sh (3 functions)
  ls.bats            # tests for commands/ls.sh
  rm.bats            # tests for commands/rm.sh
  resume.bats        # tests for commands/resume.sh
  create.bats        # tests for commands/create.sh
  claudebox.bats     # tests for the top-level dispatcher script
```

---

### Task 1: Install Prerequisites and Create Test Infrastructure

**Files:**
- Create: `tests/test_helper/common.bash`

- [ ] **Step 1: Install bats-core, bats-assert, bats-support**

Run:
```bash
brew install bats-core bats-assert bats-support
```

Expected: All three packages install successfully.

- [ ] **Step 2: Write `tests/test_helper/common.bash`**

This file provides `setup` and `teardown` functions plus helpers for creating mock binaries.

```bash
# tests/test_helper/common.bash
# Shared test infrastructure for claudebox bats tests

# Load bats helper libraries
load '/opt/homebrew/lib/bats-support/load'
load '/opt/homebrew/lib/bats-assert/load'

# Resolve SCRIPT_DIR to the repo root (two levels up from test_helper/)
SCRIPT_DIR="$(cd "$(dirname "${BATS_TEST_FILENAME}")/.." && pwd)"
export SCRIPT_DIR

setup() {
  # Create a temp directory for mock binaries and prepend to PATH
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"

  # Log file where mock docker records its invocations
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  # Install the default docker mock
  create_mock "docker"
}

teardown() {
  # Clean up mock bin dir (BATS_TEST_TMPDIR is cleaned automatically)
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

# Create a mock binary that logs calls and returns configurable output/exit code.
#
# Usage:
#   create_mock "docker"                          # default: logs args, returns 0
#   create_mock "docker" "some output" 0          # custom output and exit code
#   create_mock_script "docker" 'echo "custom"'   # fully custom script body
#
create_mock() {
  local name="$1"
  local output="${2:-}"
  local exit_code="${3:-0}"
  local log_var="MOCK_DOCKER_LOG"

  cat > "${MOCK_BIN_DIR}/${name}" <<MOCK
#!/usr/bin/env bash
echo "${name} \$*" >> "\${${log_var}}"
if [[ -n "${output}" ]]; then
  echo "${output}"
fi
exit ${exit_code}
MOCK
  chmod +x "${MOCK_BIN_DIR}/${name}"
}

# Create a mock with a fully custom script body.
# The script still has access to MOCK_DOCKER_LOG for logging.
create_mock_script() {
  local name="$1"
  local body="$2"

  cat > "${MOCK_BIN_DIR}/${name}" <<MOCK
#!/usr/bin/env bash
${body}
MOCK
  chmod +x "${MOCK_BIN_DIR}/${name}"
}
```

- [ ] **Step 3: Verify the test infrastructure works**

Create a minimal smoke test:

```bash
# Run from repo root:
cat > /tmp/smoke.bats << 'EOF'
#!/usr/bin/env bats

setup() {
  MOCK_BIN_DIR="$(mktemp -d)"
  export PATH="${MOCK_BIN_DIR}:${PATH}"
  cat > "${MOCK_BIN_DIR}/docker" << 'MOCK'
#!/usr/bin/env bash
echo "mock docker called with: $*"
MOCK
  chmod +x "${MOCK_BIN_DIR}/docker"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}"
}

@test "mock docker works" {
  run docker sandbox ls
  [ "$status" -eq 0 ]
  [[ "$output" == *"mock docker called with: sandbox ls"* ]]
}
EOF
bats /tmp/smoke.bats
```

Expected: `1 test, 0 failures`. Then delete the smoke test file.

- [ ] **Step 4: Commit**

```bash
git add tests/test_helper/common.bash
git commit -m "test: add bats test infrastructure with docker mock helpers"
```

---

### Task 2: Tests for `lib/helpers.sh`

**Files:**
- Create: `tests/helpers.bats`
- Read: `lib/helpers.sh`

- [ ] **Step 1: Write `refresh_credentials` tests**

```bash
#!/usr/bin/env bats

load 'test_helper/common'

setup() {
  # Call the common setup (creates mock dir, docker mock, etc.)
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"
  create_mock "docker"

  # Source helpers (needs SCRIPT_DIR set by common.bash)
  source "${SCRIPT_DIR}/lib/helpers.sh"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

@test "refresh_credentials: writes credentials to sandbox when keychain has them" {
  # Mock security to return a credential value
  create_mock "security" "my-secret-creds"

  run refresh_credentials "test-sandbox"

  assert_success
  assert_output --partial "Refreshing credentials"
  # Verify docker sandbox exec was called to write credentials
  assert [ -f "${MOCK_DOCKER_LOG}" ]
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "docker sandbox exec test-sandbox"
}

@test "refresh_credentials: warns when no credentials found" {
  # Mock security to fail (no keychain entry)
  create_mock "security" "" 1

  run refresh_credentials "test-sandbox"

  assert_success
  assert_output --partial "WARNING: No credentials found in Keychain"
  # Should NOT call docker sandbox exec since creds is empty
  run cat "${MOCK_DOCKER_LOG}"
  refute_output --partial "sandbox exec"
}
```

- [ ] **Step 2: Run the tests to verify they pass**

Run: `bats tests/helpers.bats`
Expected: 2 tests, 0 failures

- [ ] **Step 3: Write `setup_environment` tests**

Append to `tests/helpers.bats`:

```bash
@test "setup_environment: truncates persistent env file" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  run setup_environment "test-sandbox"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "sandbox exec test-sandbox"
  assert_output --partial "truncate"
}

@test "setup_environment: exports GITHUB_USERNAME when set" {
  export GITHUB_USERNAME="testuser"
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  run setup_environment "test-sandbox"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "GITHUB_USERNAME"
}

@test "setup_environment: skips GITHUB_USERNAME when not set" {
  unset GITHUB_USERNAME
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  run setup_environment "test-sandbox"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  refute_output --partial "GITHUB_USERNAME"
}
```

- [ ] **Step 4: Run tests**

Run: `bats tests/helpers.bats`
Expected: 5 tests, 0 failures

- [ ] **Step 5: Write `setup_environment` JVM proxy test**

Append to `tests/helpers.bats`:

```bash
@test "setup_environment: configures JVM proxy when HTTPS_PROXY is set" {
  # The JVM proxy config runs inside docker sandbox exec, so we verify
  # the exec call is made. The actual HTTPS_PROXY logic runs inside the container.
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  run setup_environment "test-sandbox"

  assert_success
  # setup_environment makes multiple docker sandbox exec calls;
  # one truncates the env file, one sets GITHUB_USERNAME (if set),
  # and one handles JVM proxy + keytool
  run cat "${MOCK_DOCKER_LOG}"
  # At least 2 docker sandbox exec calls (truncate + JVM proxy block)
  run grep -c "sandbox exec" "${MOCK_DOCKER_LOG}"
  assert [ "$output" -ge 2 ]
}
```

- [ ] **Step 6: Write `wrap_claude_binary` tests**

Append to `tests/helpers.bats`:

```bash
@test "wrap_claude_binary: calls docker sandbox exec" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  run wrap_claude_binary "test-sandbox"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "docker sandbox exec test-sandbox"
}

@test "wrap_claude_binary: idempotency — script content includes claude-real guard" {
  # The wrap_claude_binary function sends a shell script to docker sandbox exec
  # that checks 'if [ ! -f "${CLAUDE_BIN}-real" ]' before moving the binary.
  # We verify the exec call contains the guard by inspecting the logged args.
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  run wrap_claude_binary "test-sandbox"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "claude-real"
}
```

- [ ] **Step 7: Run all helpers tests**

Run: `bats tests/helpers.bats`
Expected: 8 tests, 0 failures

- [ ] **Step 8: Commit**

```bash
git add tests/helpers.bats
git commit -m "test: add unit tests for lib/helpers.sh"
```

---

### Task 3: Tests for `commands/ls.sh`

**Files:**
- Create: `tests/ls.bats`
- Read: `commands/ls.sh`

- [ ] **Step 1: Write ls tests**

```bash
#!/usr/bin/env bats

load 'test_helper/common'

setup() {
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  source "${SCRIPT_DIR}/lib/helpers.sh"
  source "${SCRIPT_DIR}/commands/ls.sh"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

@test "cmd_ls: delegates to docker sandbox ls" {
  create_mock "docker" "NAME STATUS
myapp-python-sandbox-20260320 running"

  run cmd_ls

  assert_success
  assert_output --partial "myapp-python-sandbox-20260320"
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "docker sandbox ls"
}

@test "cmd_ls: succeeds with no sandboxes" {
  create_mock "docker" ""

  run cmd_ls

  assert_success
}
```

- [ ] **Step 2: Run tests**

Run: `bats tests/ls.bats`
Expected: 2 tests, 0 failures

- [ ] **Step 3: Commit**

```bash
git add tests/ls.bats
git commit -m "test: add unit tests for commands/ls.sh"
```

---

### Task 4: Tests for `commands/rm.sh`

**Files:**
- Create: `tests/rm.bats`
- Read: `commands/rm.sh`

- [ ] **Step 1: Write rm basic tests (usage, single sandbox removal, not found)**

```bash
#!/usr/bin/env bats

load 'test_helper/common'

setup() {
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  source "${SCRIPT_DIR}/lib/helpers.sh"
  source "${SCRIPT_DIR}/commands/rm.sh"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

@test "cmd_rm: shows usage when called with no args" {
  run cmd_rm

  assert_failure
  assert_output --partial "Usage:"
  assert_output --partial "rm"
}

@test "cmd_rm: removes a named sandbox that exists" {
  # docker sandbox ls returns matching sandbox, docker sandbox rm succeeds
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "myapp-python-sandbox-20260320  running"
    fi
  '

  run cmd_rm "myapp-python-sandbox-20260320"

  assert_success
  assert_output --partial "Removed sandbox: myapp-python-sandbox-20260320"
}

@test "cmd_rm: prints error when sandbox not found" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    # sandbox ls returns no matching output
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "other-sandbox  running"
    fi
  '

  run cmd_rm "nonexistent-sandbox"

  assert_failure
  assert_output --partial "not found"
}

@test "cmd_rm: partial name match with grep -q" {
  # grep -q does substring matching, so "foo" matches "foobar"
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "foobar-sandbox  running"
    fi
  '

  run cmd_rm "foo"

  assert_success
  assert_output --partial "Removed sandbox: foo"
}

@test "cmd_rm: docker sandbox rm failure — still prints Removed when no set -e" {
  # Note: cmd_rm does not use set -e, so docker sandbox rm failure does not
  # stop execution. The "Removed" message still prints. When called through
  # the claudebox dispatcher (which has set -euo pipefail), this would fail.
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "mysandbox  running"
    elif [[ "$1 $2" == "sandbox rm" ]]; then
      echo "Error: sandbox busy" >&2
      exit 1
    fi
  '

  run cmd_rm "mysandbox"

  # Function returns 0 because rm.sh has no set -e — the docker rm exit code is lost
  assert_success
  assert_output --partial "Removed sandbox: mysandbox"
}
```

- [ ] **Step 2: Run tests**

Run: `bats tests/rm.bats`
Expected: 5 tests, 0 failures

- [ ] **Step 3: Write `rm all` tests**

Append to `tests/rm.bats`:

```bash
@test "cmd_rm all: removes only sandboxes matching current workspace name" {
  # cd to a temp dir with a known basename
  cd "${BATS_TEST_TMPDIR}"
  mkdir -p myproject && cd myproject

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "myproject-python-sandbox-20260320  running\nmyproject-jvm-sandbox-20260321  running\nother-project-sandbox  running\n"
    fi
  '

  run cmd_rm "all"

  assert_success
  assert_output --partial "Removed 2 sandbox(es)"
  # Verify the other-project sandbox was NOT removed
  run cat "${MOCK_DOCKER_LOG}"
  refute_output --partial "other-project-sandbox"
}

@test "cmd_rm all: prints message when no sandboxes found" {
  cd "${BATS_TEST_TMPDIR}"
  mkdir -p emptyproject && cd emptyproject

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "other-project-sandbox  running"
    fi
  '

  run cmd_rm "all"

  assert_success
  assert_output --partial "No sandboxes found"
}

@test "cmd_rm all: counts attempted removals including failures" {
  cd "${BATS_TEST_TMPDIR}"
  mkdir -p failproject && cd failproject

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "failproject-sandbox-1  running\nfailproject-sandbox-2  running\n"
    elif [[ "$1 $2" == "sandbox rm" ]]; then
      exit 1
    fi
  '

  run cmd_rm "all"

  assert_success
  # Count includes failures because increment is after || true
  assert_output --partial "Removed 2 sandbox(es)"
}
```

- [ ] **Step 4: Run all rm tests**

Run: `bats tests/rm.bats`
Expected: 8 tests, 0 failures

- [ ] **Step 5: Commit**

```bash
git add tests/rm.bats
git commit -m "test: add unit tests for commands/rm.sh"
```

---

### Task 5: Tests for `commands/resume.sh`

**Files:**
- Create: `tests/resume.bats`
- Read: `commands/resume.sh`

- [ ] **Step 1: Write error-path tests (no sandboxes, unknown args)**

```bash
#!/usr/bin/env bats

load 'test_helper/common'

setup() {
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  source "${SCRIPT_DIR}/lib/helpers.sh"
  source "${SCRIPT_DIR}/commands/resume.sh"

  # Default: cd to a directory with a known basename for workspace matching
  cd "${BATS_TEST_TMPDIR}"
  mkdir -p testproject && cd testproject
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

@test "cmd_resume: errors when no sandboxes exist for workspace" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "NAME STATUS"
    fi
  '

  run cmd_resume

  assert_failure
  assert_output --partial "No sandboxes found"
}

@test "cmd_resume: errors on unknown arguments" {
  run cmd_resume --invalid

  assert_failure
  assert_output --partial "Unknown argument"
}
```

- [ ] **Step 2: Run tests**

Run: `bats tests/resume.bats`
Expected: 2 tests, 0 failures

- [ ] **Step 3: Write single-sandbox tests (accept and decline)**

Append to `tests/resume.bats`:

```bash
@test "cmd_resume: auto-selects single sandbox when user confirms" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-20260320  running\n"
    fi
  '

  # Pipe "Y" to stdin for the confirmation prompt
  run bash -c 'source "${SCRIPT_DIR}/lib/helpers.sh"; source "${SCRIPT_DIR}/commands/resume.sh"; cd "${BATS_TEST_TMPDIR}/testproject"; echo "Y" | cmd_resume'

  assert_success
  assert_output --partial "Resuming sandbox: testproject-python-sandbox-20260320"
  assert_output --partial "Starting sandbox"
}

@test "cmd_resume: exits cleanly when user declines" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-20260320  running\n"
    fi
  '

  run bash -c 'source "${SCRIPT_DIR}/lib/helpers.sh"; source "${SCRIPT_DIR}/commands/resume.sh"; cd "${BATS_TEST_TMPDIR}/testproject"; echo "n" | cmd_resume'

  assert_success
  # Should NOT contain "Resuming sandbox" since user declined
  refute_output --partial "Resuming sandbox"
}
```

- [ ] **Step 4: Run tests**

Run: `bats tests/resume.bats`
Expected: 4 tests, 0 failures

- [ ] **Step 5: Write multi-sandbox picker test**

Append to `tests/resume.bats`:

```bash
@test "cmd_resume: picker selects correct sandbox from multiple" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-1  running\ntestproject-jvm-sandbox-2  running\n"
    fi
  '

  # Pick option 2
  run bash -c 'source "${SCRIPT_DIR}/lib/helpers.sh"; source "${SCRIPT_DIR}/commands/resume.sh"; cd "${BATS_TEST_TMPDIR}/testproject"; echo "2" | cmd_resume'

  assert_success
  assert_output --partial "Resuming sandbox: testproject-jvm-sandbox-2"
}

@test "cmd_resume: picker rejects invalid input and re-prompts" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-1  running\ntestproject-jvm-sandbox-2  running\n"
    fi
  '

  # Send invalid input "99" first, then valid "1"
  run bash -c 'source "${SCRIPT_DIR}/lib/helpers.sh"; source "${SCRIPT_DIR}/commands/resume.sh"; cd "${BATS_TEST_TMPDIR}/testproject"; printf "99\n1\n" | cmd_resume'

  assert_success
  assert_output --partial "Invalid selection"
  assert_output --partial "Resuming sandbox: testproject-python-sandbox-1"
}

@test "cmd_resume: calls setup_environment, refresh_credentials, wrap_claude_binary" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-1  running\n"
    fi
  '
  create_mock "security" "" 1

  run bash -c 'source "${SCRIPT_DIR}/lib/helpers.sh"; source "${SCRIPT_DIR}/commands/resume.sh"; cd "${BATS_TEST_TMPDIR}/testproject"; echo "Y" | cmd_resume'

  assert_success
  # setup_environment truncates env file
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "sandbox exec"
  # refresh_credentials prints this message
  run bash -c 'source "${SCRIPT_DIR}/lib/helpers.sh"; source "${SCRIPT_DIR}/commands/resume.sh"; cd "${BATS_TEST_TMPDIR}/testproject"; echo "Y" | cmd_resume 2>&1'
  assert_output --partial "Refreshing credentials"
}

@test "cmd_resume: passes agent args through to docker sandbox run" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-1  running\n"
    fi
  '

  run bash -c 'source "${SCRIPT_DIR}/lib/helpers.sh"; source "${SCRIPT_DIR}/commands/resume.sh"; cd "${BATS_TEST_TMPDIR}/testproject"; echo "Y" | cmd_resume -- -p "fix the tests"'

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "sandbox run"
  assert_output --partial -- "-p"
  assert_output --partial "fix the tests"
}
```

- [ ] **Step 6: Run all resume tests**

Run: `bats tests/resume.bats`
Expected: 8 tests, 0 failures

- [ ] **Step 7: Commit**

```bash
git add tests/resume.bats
git commit -m "test: add unit tests for commands/resume.sh"
```

---

### Task 6: Tests for `commands/create.sh`

**Files:**
- Create: `tests/create.bats`
- Read: `commands/create.sh`

This is the most complex command. The docker mock needs to handle multiple different `docker` subcommands (build, sandbox create, sandbox exec, sandbox network, sandbox run) in a single test.

- [ ] **Step 1: Write setup and missing-Dockerfile test**

```bash
#!/usr/bin/env bats

load 'test_helper/common'

setup() {
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  # Mock date for deterministic sandbox names
  create_mock "date" "20260323-120000"
  # Mock security (used by refresh_credentials)
  create_mock "security" "" 1

  source "${SCRIPT_DIR}/lib/helpers.sh"
  source "${SCRIPT_DIR}/commands/create.sh"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

@test "cmd_create: fails when template has no Dockerfile" {
  # Point SCRIPT_DIR to a temp directory with no Dockerfile
  SCRIPT_DIR="${BATS_TEST_TMPDIR}"
  mkdir -p "${SCRIPT_DIR}/badtemplate"
  # No Dockerfile created

  run cmd_create "badtemplate"

  assert_failure
  assert_output --partial "Error: No Dockerfile found"
}
```

- [ ] **Step 2: Run test**

Run: `bats tests/create.bats`
Expected: 1 test, 0 failures

- [ ] **Step 3: Write the happy-path test (no network policy)**

Append to `tests/create.bats`:

```bash
@test "cmd_create: builds image and creates sandbox with correct names" {
  # Set up a fake template with Dockerfile but no allowed-hosts.txt
  local FAKE_DIR="${BATS_TEST_TMPDIR}/fakerepo"
  mkdir -p "${FAKE_DIR}/mytemplate"
  touch "${FAKE_DIR}/mytemplate/Dockerfile"
  SCRIPT_DIR="${FAKE_DIR}"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  cd "${BATS_TEST_TMPDIR}"
  mkdir -p myworkspace && cd myworkspace

  run cmd_create "mytemplate"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  # Verify docker build
  assert_output --partial "docker build -t mytemplate-sandbox ${FAKE_DIR}/mytemplate"
  # Verify docker sandbox create
  assert_output --partial "docker sandbox create -t mytemplate-sandbox --name myworkspace-mytemplate-sandbox-20260323-120000"
  # Verify docker sandbox run
  assert_output --partial "docker sandbox run myworkspace-mytemplate-sandbox-20260323-120000 -- --dangerously-skip-permissions"
}

@test "cmd_create: skips network policy when no allowed-hosts.txt" {
  local FAKE_DIR="${BATS_TEST_TMPDIR}/fakerepo2"
  mkdir -p "${FAKE_DIR}/mytemplate"
  touch "${FAKE_DIR}/mytemplate/Dockerfile"
  SCRIPT_DIR="${FAKE_DIR}"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  cd "${BATS_TEST_TMPDIR}"
  mkdir -p workspace2 && cd workspace2

  run cmd_create "mytemplate"

  assert_success
  assert_output --partial "No allowed-hosts.txt found"
  run cat "${MOCK_DOCKER_LOG}"
  refute_output --partial "sandbox network proxy"
}
```

- [ ] **Step 4: Run tests**

Run: `bats tests/create.bats`
Expected: 3 tests, 0 failures

- [ ] **Step 5: Write workspace argument and agent args tests**

Append to `tests/create.bats`:

```bash
@test "cmd_create: parses custom workspace argument" {
  local FAKE_DIR="${BATS_TEST_TMPDIR}/fakerepo3"
  mkdir -p "${FAKE_DIR}/mytemplate"
  touch "${FAKE_DIR}/mytemplate/Dockerfile"
  SCRIPT_DIR="${FAKE_DIR}"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  mkdir -p "${BATS_TEST_TMPDIR}/custom-workspace"

  run cmd_create "mytemplate" "${BATS_TEST_TMPDIR}/custom-workspace"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "docker sandbox create -t mytemplate-sandbox --name custom-workspace-mytemplate-sandbox-20260323-120000"
}

@test "cmd_create: passes agent args after -- to docker sandbox run" {
  local FAKE_DIR="${BATS_TEST_TMPDIR}/fakerepo4"
  mkdir -p "${FAKE_DIR}/mytemplate"
  touch "${FAKE_DIR}/mytemplate/Dockerfile"
  SCRIPT_DIR="${FAKE_DIR}"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  cd "${BATS_TEST_TMPDIR}"
  mkdir -p ws4 && cd ws4

  run cmd_create "mytemplate" -- -p "fix the tests"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial -- "-p fix the tests"
}
```

- [ ] **Step 6: Run tests**

Run: `bats tests/create.bats`
Expected: 5 tests, 0 failures

- [ ] **Step 7: Write network policy and firewall verification tests**

Append to `tests/create.bats`:

```bash
@test "cmd_create: symlinks host config and copies workspace" {
  local FAKE_DIR="${BATS_TEST_TMPDIR}/fakerepo7"
  mkdir -p "${FAKE_DIR}/mytemplate"
  touch "${FAKE_DIR}/mytemplate/Dockerfile"
  SCRIPT_DIR="${FAKE_DIR}"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  cd "${BATS_TEST_TMPDIR}"
  mkdir -p ws7 && cd ws7

  run cmd_create "mytemplate"

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  # Verify symlink calls for host config
  assert_output --partial "ln -sf"
  assert_output --partial ".claude.json"
  assert_output --partial "settings.json"
  assert_output --partial "plugins"
  # Verify workspace copy
  assert_output --partial "cp -a"
  assert_output --partial "git checkout -b"
}

@test "cmd_create: calls setup_environment, refresh_credentials, wrap_claude_binary" {
  local FAKE_DIR="${BATS_TEST_TMPDIR}/fakerepo8"
  mkdir -p "${FAKE_DIR}/mytemplate"
  touch "${FAKE_DIR}/mytemplate/Dockerfile"
  SCRIPT_DIR="${FAKE_DIR}"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
  '

  cd "${BATS_TEST_TMPDIR}"
  mkdir -p ws8 && cd ws8

  run cmd_create "mytemplate"

  assert_success
  # refresh_credentials prints this
  assert_output --partial "Refreshing credentials"
  # setup_environment calls docker sandbox exec with truncate
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "truncate"
  # wrap_claude_binary calls docker sandbox exec with claude-real guard
  assert_output --partial "claude-real"
}

@test "cmd_create: applies network policy when allowed-hosts.txt exists" {
  local FAKE_DIR="${BATS_TEST_TMPDIR}/fakerepo5"
  mkdir -p "${FAKE_DIR}/nettemplate"
  touch "${FAKE_DIR}/nettemplate/Dockerfile"
  # Create allowed-hosts.txt with comments and blank lines
  cat > "${FAKE_DIR}/nettemplate/allowed-hosts.txt" <<'EOF'
# Package registries
pypi.org
files.pythonhosted.org

# GitHub API
api.github.com
EOF
  SCRIPT_DIR="${FAKE_DIR}"

  # Mock docker: curl to example.com fails (exit 1), curl to api.github.com succeeds (exit 0)
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$*" == *"curl"*"example.com"* ]]; then
      exit 1
    fi
  '

  cd "${BATS_TEST_TMPDIR}"
  mkdir -p ws5 && cd ws5

  run cmd_create "nettemplate"

  assert_success
  assert_output --partial "Applying network policy"
  assert_output --partial "3 hosts allowed"
  assert_output --partial "Network policy verified"
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "sandbox network proxy"
  assert_output --partial "--allow-host pypi.org"
  assert_output --partial "--allow-host api.github.com"
}

@test "cmd_create: aborts when firewall verification fails (blocked host reachable)" {
  local FAKE_DIR="${BATS_TEST_TMPDIR}/fakerepo6"
  mkdir -p "${FAKE_DIR}/nettemplate"
  touch "${FAKE_DIR}/nettemplate/Dockerfile"
  echo "api.github.com" > "${FAKE_DIR}/nettemplate/allowed-hosts.txt"
  SCRIPT_DIR="${FAKE_DIR}"

  # Mock docker: curl to example.com SUCCEEDS (bad — firewall is broken)
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    # All commands succeed including the curl to example.com
  '

  cd "${BATS_TEST_TMPDIR}"
  mkdir -p ws6 && cd ws6

  run cmd_create "nettemplate"

  assert_failure
  assert_output --partial "ERROR: Firewall verification failed"
  assert_output --partial "was able to reach"
}
```

- [ ] **Step 8: Run all create tests**

Run: `bats tests/create.bats`
Expected: 9 tests, 0 failures

- [ ] **Step 9: Commit**

```bash
git add tests/create.bats
git commit -m "test: add unit tests for commands/create.sh"
```

---

### Task 7: Tests for the Dispatcher (`claudebox`)

**Files:**
- Create: `tests/claudebox.bats`
- Read: `claudebox`

The dispatcher is tested by running the `claudebox` script as a subprocess. We need the docker mock on PATH for the commands it dispatches to.

- [ ] **Step 1: Write dispatcher tests**

```bash
#!/usr/bin/env bats

load 'test_helper/common'

setup() {
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"
  create_mock "docker"

  CLAUDEBOX="${SCRIPT_DIR}/claudebox"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

@test "claudebox: shows usage and exits 1 with no args" {
  run "${CLAUDEBOX}"

  assert_failure
  assert_output --partial "Usage:"
  assert_output --partial "Available templates:"
}

@test "claudebox: lists available templates in usage" {
  run "${CLAUDEBOX}"

  assert_failure
  # python and jvm templates should be listed (they have Dockerfiles)
  assert_output --partial "python"
  assert_output --partial "jvm"
}

@test "claudebox: routes ls to cmd_ls" {
  create_mock "docker" "NAME STATUS
test-sandbox  running"

  run "${CLAUDEBOX}" ls

  assert_success
  assert_output --partial "test-sandbox"
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "docker sandbox ls"
}

@test "claudebox: routes rm to cmd_rm" {
  run "${CLAUDEBOX}" rm

  assert_failure
  assert_output --partial "Usage:"
  assert_output --partial "rm"
}

@test "claudebox: routes unknown command to cmd_create (template mode)" {
  # "python" is a valid template — it has a Dockerfile and allowed-hosts.txt.
  # The docker mock must return exit 1 for curl-to-example.com (firewall check)
  # and exit 0 for curl-to-api.github.com, otherwise firewall verification fails.
  create_mock "date" "20260323-120000"
  create_mock "security" "" 1
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$*" == *"curl"*"example.com"* ]]; then
      exit 1
    fi
  '

  run "${CLAUDEBOX}" python

  assert_success
  run cat "${MOCK_DOCKER_LOG}"
  assert_output --partial "docker build -t python-sandbox"
}
```

- [ ] **Step 2: Run tests**

Run: `bats tests/claudebox.bats`
Expected: 5 tests, 0 failures

- [ ] **Step 3: Commit**

```bash
git add tests/claudebox.bats
git commit -m "test: add unit tests for claudebox dispatcher"
```

---

### Task 8: Final Verification

- [ ] **Step 1: Run the full test suite**

Run: `bats tests/`
Expected: All tests pass (approximately 34 tests, 0 failures).

- [ ] **Step 2: Fix any failures**

If any tests fail, debug and fix them. Re-run `bats tests/` until all pass.

- [ ] **Step 3: Final commit (if any fixes were needed)**

```bash
git add tests/
git commit -m "test: fix test issues from full suite run"
```
