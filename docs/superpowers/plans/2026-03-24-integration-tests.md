# Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add integration tests that verify Docker sandbox construction, and restructure the test directory into `tests/unit/` and `tests/integration/`.

**Architecture:** Move existing unit tests into `tests/unit/`, refactor shared test helpers into `common.bash` (bats libs + SCRIPT_DIR), `unit.bash` (mock infrastructure), and `integration.bash` (real Docker sandbox operations). Integration tests create real Docker sandboxes and verify construction without running Claude Code.

**Tech Stack:** BATS v1.11.1, Docker Desktop with sandbox support, bash

**Spec:** `docs/superpowers/specs/2026-03-24-integration-tests-design.md`

---

### Task 1: Refactor test helpers into common.bash + unit.bash

**Files:**
- Modify: `tests/test_helper/common.bash`
- Create: `tests/test_helper/unit.bash`

- [ ] **Step 1: Rewrite common.bash as minimal shared infrastructure**

Replace the entire file with:

```bash
# tests/test_helper/common.bash
# Shared test infrastructure — bats libraries and repo root resolution

# Resolve SCRIPT_DIR to the repo root (two levels up from tests/{unit,integration}/)
SCRIPT_DIR="$(cd "$(dirname "${BATS_TEST_FILENAME}")/../.." && pwd)"
export SCRIPT_DIR

# Load bats helper libraries via absolute paths
load "${SCRIPT_DIR}/tests/test_helper/bats-support/load"
load "${SCRIPT_DIR}/tests/test_helper/bats-assert/load"
```

- [ ] **Step 2: Create unit.bash with mock infrastructure**

```bash
# tests/test_helper/unit.bash
# Unit test infrastructure — mock binaries and shared setup/teardown

source "$(dirname "${BASH_SOURCE[0]}")/common.bash"

# Base setup for unit tests: mock bin directory, docker call log, default docker mock.
# Call from each test file's setup() function.
common_setup() {
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"

  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  create_mock "docker"
}

# Base teardown for unit tests.
common_teardown() {
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

  cat > "${MOCK_BIN_DIR}/${name}" <<MOCK
#!/usr/bin/env bash
echo "${name} \$*" >> "\${MOCK_DOCKER_LOG}"
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

- [ ] **Step 3: Commit**

```bash
git add tests/test_helper/common.bash tests/test_helper/unit.bash
git commit -m "refactor: split test helpers into common.bash and unit.bash"
```

---

### Task 2: Move unit tests to tests/unit/ and update loading

**Files:**
- Move: all `tests/*.bats` → `tests/unit/*.bats`
- Modify: all 6 test files (update load paths, remove duplicated helpers)

- [ ] **Step 1: Create tests/unit/ and move test files**

```bash
mkdir -p tests/unit
git mv tests/claudebox.bats tests/unit/
git mv tests/create.bats tests/unit/
git mv tests/helpers.bats tests/unit/
git mv tests/ls.bats tests/unit/
git mv tests/resume.bats tests/unit/
git mv tests/rm.bats tests/unit/
```

- [ ] **Step 2: Update tests/unit/claudebox.bats**

Replace the load statement and setup/teardown at the top of the file. Change:

```bash
load 'test_helper/common'

setup() {
  # Create a temp directory for mock binaries and prepend to PATH
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"

  # Log file where mock docker records its invocations
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  # Resolve repo root from this file's location
  SCRIPT_DIR="$(cd "$(dirname "${BATS_TEST_FILENAME}")/.." && pwd)"
  export SCRIPT_DIR

  # Path to the claudebox script under test
  CLAUDEBOX="${SCRIPT_DIR}/claudebox"

  # Install default docker mock (logs invocations, exits 0)
  create_mock "docker"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}
```

To:

```bash
load "../test_helper/unit"

setup() {
  common_setup
  CLAUDEBOX="${SCRIPT_DIR}/claudebox"
}

teardown() {
  common_teardown
}
```

All `@test` blocks remain unchanged.

- [ ] **Step 3: Update tests/unit/create.bats**

Replace lines 1–75 (load statements, setup/teardown, inline create_mock/create_mock_script) with:

```bash
#!/usr/bin/env bats
# tests/unit/create.bats — unit tests for commands/create.sh

load "../test_helper/unit"

# ---------------------------------------------------------------------------
# Setup / Teardown
# ---------------------------------------------------------------------------

setup() {
  common_setup

  # Mock date to return a fixed value for deterministic sandbox names
  create_mock "date" "20260323-120000"

  # Mock security to fail (skip credential writing)
  create_mock "security" "" 1

  # Source the files under test
  # shellcheck source=../../src/lib/helpers.sh
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  # shellcheck source=../../src/commands/create.sh
  source "${SCRIPT_DIR}/src/commands/create.sh"
}

teardown() {
  common_teardown
}
```

Keep the `setup_fake_template()` helper (lines 83–89 in original) and all `@test` blocks unchanged.

- [ ] **Step 4: Update tests/unit/helpers.bats**

Replace lines 1–65 (load statements, setup/teardown, inline create_mock/create_mock_script) with:

```bash
#!/usr/bin/env bats
# tests/unit/helpers.bats — unit tests for lib/helpers.sh

load "../test_helper/unit"

# ---------------------------------------------------------------------------
# Setup / Teardown
# ---------------------------------------------------------------------------

setup() {
  common_setup

  # Source the file under test
  # shellcheck source=../../src/lib/helpers.sh
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
}

teardown() {
  common_teardown
}
```

All `@test` blocks remain unchanged.

- [ ] **Step 5: Update tests/unit/ls.bats**

Replace lines 1–69 (load statements, setup/teardown, inline create_mock/create_mock_script) with:

```bash
#!/usr/bin/env bats
# tests/unit/ls.bats — unit tests for commands/ls.sh

load "../test_helper/unit"

# ---------------------------------------------------------------------------
# Setup / Teardown
# ---------------------------------------------------------------------------

setup() {
  common_setup

  # Source the files under test
  # shellcheck source=../../src/lib/helpers.sh
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  # shellcheck source=../../src/commands/ls.sh
  source "${SCRIPT_DIR}/src/commands/ls.sh"
}

teardown() {
  common_teardown
}
```

All `@test` blocks remain unchanged.

- [ ] **Step 6: Update tests/unit/resume.bats**

Replace lines 1–77 (load statements, setup/teardown, inline create_mock/create_mock_script) with:

```bash
#!/usr/bin/env bats
# tests/unit/resume.bats — unit tests for commands/resume.sh

load "../test_helper/unit"

# ---------------------------------------------------------------------------
# Setup / Teardown
# ---------------------------------------------------------------------------

setup() {
  common_setup

  # Create a testproject workspace directory
  mkdir -p "${BATS_TEST_TMPDIR}/testproject"

  # Install docker mock with sandbox ls support (single sandbox by default)
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-1  running\n"
    fi
  '

  # Source the files under test
  # shellcheck source=../../src/lib/helpers.sh
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  # shellcheck source=../../src/commands/resume.sh
  source "${SCRIPT_DIR}/src/commands/resume.sh"
}

teardown() {
  common_teardown
}
```

All `@test` blocks remain unchanged.

- [ ] **Step 7: Update tests/unit/rm.bats**

Replace lines 1–69 (load statements, setup/teardown, inline create_mock/create_mock_script) with:

```bash
#!/usr/bin/env bats
# tests/unit/rm.bats — unit tests for commands/rm.sh

load "../test_helper/unit"

# ---------------------------------------------------------------------------
# Setup / Teardown
# ---------------------------------------------------------------------------

setup() {
  common_setup

  # Source the files under test
  # shellcheck source=../../src/lib/helpers.sh
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  # shellcheck source=../../src/commands/rm.sh
  source "${SCRIPT_DIR}/src/commands/rm.sh"
}

teardown() {
  common_teardown
}
```

All `@test` blocks remain unchanged.

- [ ] **Step 8: Run unit tests to verify migration**

Run: `./tests/bats/bin/bats tests/unit/*.bats`

Expected: All 39 tests pass (same count as before migration).

- [ ] **Step 9: Commit**

```bash
git add tests/unit/ tests/test_helper/
git commit -m "refactor: move unit tests to tests/unit/ and deduplicate helpers"
```

---

### Task 3: Update Makefile

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Update Makefile with new targets**

Replace entire content:

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

- [ ] **Step 2: Verify make test works**

Run: `make test`

Expected: All unit tests pass.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "chore: update Makefile with unit/integration test targets"
```

---

### Task 4: Create integration test infrastructure

**Files:**
- Create: `tests/test_helper/integration.bash`
- Create: `tests/integration/` directory

- [ ] **Step 1: Create tests/integration/ directory**

```bash
mkdir -p tests/integration
```

- [ ] **Step 2: Create integration.bash helper**

```bash
# tests/test_helper/integration.bash
# Integration test infrastructure — real Docker sandbox operations

source "$(dirname "${BASH_SOURCE[0]}")/common.bash"

# Source helpers from production code
source "${SCRIPT_DIR}/src/lib/helpers.sh"

# Skip all tests if docker sandbox is not available.
# Call from setup_file() or individual tests.
require_docker_sandbox() {
  if ! command -v docker &>/dev/null; then
    skip "docker not found"
  fi
  if ! docker sandbox ls &>/dev/null 2>&1; then
    skip "docker sandbox not available — requires Docker Desktop with sandbox support"
  fi
}

# Build a template image (quiet mode).
# Usage: build_template_image "python"
build_template_image() {
  local template="$1"
  docker build -q -t "${template}-sandbox" "${SCRIPT_DIR}/templates/${template}"
}

# Create a test sandbox — replicates cmd_create steps 2-4 without running the sandbox.
# Requires image to be built first via build_template_image.
# Usage: create_test_sandbox "python" "/path/to/workspace"
# Sets: SANDBOX_NAME (exported)
create_test_sandbox() {
  local template="$1"
  local workspace="$2"
  local image_name="${template}-sandbox"
  local workspace_name
  workspace_name="$(printf '%s' "$(basename "${workspace}")" | tr -cs 'a-zA-Z0-9_.-' '-')"
  local session_id="sandbox-$(date +%Y%m%d-%H%M%S)"
  SANDBOX_NAME="${workspace_name}-${template}-${session_id}"
  export SANDBOX_NAME

  local host_claude_dir="${HOME}/.claude"
  docker sandbox create -t "${image_name}" --name "${SANDBOX_NAME}" claude "${workspace}" "${host_claude_dir}"

  # Symlink host config
  docker sandbox exec "${SANDBOX_NAME}" ln -sf "${host_claude_dir}/.claude.json" /home/agent/.claude.json
  docker sandbox exec "${SANDBOX_NAME}" ln -sf "${host_claude_dir}/settings.json" /home/agent/.claude/settings.json
  docker sandbox exec "${SANDBOX_NAME}" ln -sf "${host_claude_dir}/plugins" /home/agent/.claude/plugins

  # Copy workspace to container-local filesystem and create branch
  docker sandbox exec "${SANDBOX_NAME}" sh -c "
    cp -a '${workspace}/.' /home/agent/workspace/
    cd /home/agent/workspace
    git clean -fdx -q
    git checkout -b '${session_id}'
  "

  setup_environment "${SANDBOX_NAME}"
  wrap_claude_binary "${SANDBOX_NAME}"
}

# Apply network policy from a template's allowed-hosts.txt.
# Usage: apply_network_policy "python"
apply_network_policy() {
  local template="$1"
  local hosts_file="${SCRIPT_DIR}/templates/${template}/allowed-hosts.txt"
  if [[ -f "${hosts_file}" ]]; then
    local proxy_args=(--policy deny)
    while IFS= read -r host || [[ -n "$host" ]]; do
      [[ -z "$host" || "$host" == \#* ]] && continue
      proxy_args+=(--allow-host "$host")
    done < "${hosts_file}"
    docker sandbox network proxy "${SANDBOX_NAME}" "${proxy_args[@]}"
  fi
}

# Remove a test sandbox (silent on failure).
# Usage: cleanup_test_sandbox [name]
cleanup_test_sandbox() {
  local name="${1:-${SANDBOX_NAME:-}}"
  [[ -n "$name" ]] && docker sandbox rm "$name" 2>/dev/null || true
}

# Create a minimal git repo in a temporary directory.
# Usage: create_test_workspace "dirname"
# Returns: path via stdout
create_test_workspace() {
  local dirname="$1"
  local workspace="${BATS_FILE_TMPDIR}/${dirname}"
  mkdir -p "$workspace"
  git -C "$workspace" init -q
  echo "test content" > "$workspace/testfile.txt"
  git -C "$workspace" add .
  git -C "$workspace" commit -q -m "init"
  echo "$workspace"
}
```

- [ ] **Step 3: Commit**

```bash
git add tests/test_helper/integration.bash
git commit -m "feat: add integration test infrastructure"
```

---

### Task 5: Write integration/create.bats

**Files:**
- Create: `tests/integration/create.bats`

**Prereqs:** Docker Desktop running with sandbox support.

- [ ] **Step 1: Write create.bats**

```bash
#!/usr/bin/env bats
# tests/integration/create.bats — integration tests for image building and sandbox creation

load "../test_helper/integration"

teardown() {
  # Safety net: clean up any sandbox created during this test
  cleanup_test_sandbox 2>/dev/null || true
}

@test "python template image builds successfully" {
  require_docker_sandbox
  run build_template_image "python"
  assert_success
}

@test "jvm template image builds successfully" {
  require_docker_sandbox
  run build_template_image "jvm"
  assert_success
}

@test "sandbox is created with correct name format" {
  require_docker_sandbox

  local workspace
  workspace="$(create_test_workspace "claudebox-inttest-create-$$")"

  build_template_image "python"
  create_test_sandbox "python" "$workspace"

  # Verify name matches: <workspace>-python-sandbox-YYYYMMDD-HHMMSS
  [[ "$SANDBOX_NAME" =~ ^claudebox-inttest-create-[0-9]+-python-sandbox-[0-9]{8}-[0-9]{6}$ ]]
}
```

- [ ] **Step 2: Run the test**

Run: `./tests/bats/bin/bats tests/integration/create.bats`

Expected: All 3 tests pass (or skip if docker sandbox not available).

- [ ] **Step 3: Commit**

```bash
git add tests/integration/create.bats
git commit -m "test: add integration tests for image building and sandbox creation"
```

---

### Task 6: Write integration/filesystem.bats

**Files:**
- Create: `tests/integration/filesystem.bats`

- [ ] **Step 1: Write filesystem.bats**

```bash
#!/usr/bin/env bats
# tests/integration/filesystem.bats — integration tests for sandbox file layout

load "../test_helper/integration"

setup_file() {
  require_docker_sandbox

  INTTEST_WORKSPACE="$(create_test_workspace "claudebox-inttest-fs-$$")"
  export INTTEST_WORKSPACE

  build_template_image "python"
  create_test_sandbox "python" "$INTTEST_WORKSPACE"
}

teardown_file() {
  cleanup_test_sandbox
}

@test "repo files exist at /home/agent/workspace" {
  run docker sandbox exec "$SANDBOX_NAME" test -f /home/agent/workspace/testfile.txt
  assert_success
}

@test "git branch matching sandbox-* is checked out" {
  run docker sandbox exec "$SANDBOX_NAME" git -C /home/agent/workspace branch --show-current
  assert_success
  assert_output --regexp "^sandbox-[0-9]{8}-[0-9]{6}$"
}

@test "claude config symlinks exist" {
  run docker sandbox exec "$SANDBOX_NAME" test -L /home/agent/.claude.json
  assert_success
}

@test "claude binary wrapper contains cd /home/agent/workspace" {
  run docker sandbox exec "$SANDBOX_NAME" sh -c 'cat "$(which claude)"'
  assert_success
  assert_output --partial "cd /home/agent/workspace"
}
```

- [ ] **Step 2: Run the test**

Run: `./tests/bats/bin/bats tests/integration/filesystem.bats`

Expected: All 4 tests pass (or skip if docker sandbox not available).

- [ ] **Step 3: Commit**

```bash
git add tests/integration/filesystem.bats
git commit -m "test: add integration tests for sandbox file layout"
```

---

### Task 7: Write integration/network.bats

**Files:**
- Create: `tests/integration/network.bats`

- [ ] **Step 1: Write network.bats**

```bash
#!/usr/bin/env bats
# tests/integration/network.bats — integration tests for network policy enforcement

load "../test_helper/integration"

setup_file() {
  require_docker_sandbox

  INTTEST_WORKSPACE="$(create_test_workspace "claudebox-inttest-net-$$")"
  export INTTEST_WORKSPACE

  build_template_image "python"
  create_test_sandbox "python" "$INTTEST_WORKSPACE"
  apply_network_policy "python"
}

teardown_file() {
  cleanup_test_sandbox
}

@test "disallowed host is blocked" {
  run docker sandbox exec "$SANDBOX_NAME" curl --connect-timeout 5 -sf https://example.com
  assert_failure
}

@test "allowed host is reachable" {
  run docker sandbox exec "$SANDBOX_NAME" curl --connect-timeout 10 -sf https://api.github.com/zen
  assert_success
}

@test "sandbox without allowed-hosts.txt has unrestricted access" {
  # Create a temporary template with no allowed-hosts.txt
  local tmp_template="${BATS_FILE_TMPDIR}/nofilter-template"
  mkdir -p "$tmp_template"
  cp "${SCRIPT_DIR}/templates/python/Dockerfile" "$tmp_template/Dockerfile"

  # Build image from temporary template
  docker build -q -t "nofilter-sandbox" "$tmp_template"

  # Create a minimal sandbox (no full setup needed — just network access test)
  local nofilter_name="inttest-nofilter-$$-sandbox-$(date +%Y%m%d-%H%M%S)"
  docker sandbox create -t "nofilter-sandbox" --name "$nofilter_name" claude "$INTTEST_WORKSPACE" "${HOME}/.claude"

  # No network policy applied — example.com should be reachable
  run docker sandbox exec "$nofilter_name" curl --connect-timeout 10 -sf https://example.com

  # Clean up before asserting (so sandbox and image are removed even on failure)
  docker sandbox rm "$nofilter_name" 2>/dev/null || true
  docker rmi "nofilter-sandbox" 2>/dev/null || true

  assert_success
}
```

- [ ] **Step 2: Run the test**

Run: `./tests/bats/bin/bats tests/integration/network.bats`

Expected: All 3 tests pass (or skip if docker sandbox not available).

- [ ] **Step 3: Commit**

```bash
git add tests/integration/network.bats
git commit -m "test: add integration tests for network policy enforcement"
```

---

### Task 8: Write integration/zz_cleanup.bats

**Files:**
- Create: `tests/integration/zz_cleanup.bats`

Named `zz_` so it runs last in alphabetical glob order, after other integration test files have cleaned up their own sandboxes.

- [ ] **Step 1: Write zz_cleanup.bats**

```bash
#!/usr/bin/env bats
# tests/integration/zz_cleanup.bats — integration tests for sandbox removal
# Named zz_ to run last, after other test files have cleaned up.

load "../test_helper/integration"

# Source rm.sh for cmd_rm
source "${SCRIPT_DIR}/src/commands/rm.sh"

setup_file() {
  require_docker_sandbox

  CLEANUP_WORKSPACE="$(create_test_workspace "claudebox-inttest-cleanup-$$")"
  export CLEANUP_WORKSPACE

  build_template_image "python"
}

teardown_file() {
  # Safety net: remove any remaining test sandboxes
  local prefix
  prefix="$(printf '%s' "$(basename "${CLEANUP_WORKSPACE}")" | tr -cs 'a-zA-Z0-9_.-' '-')"
  docker sandbox ls 2>/dev/null | grep -oE "^${prefix}-[^ ]+" | while read -r name; do
    docker sandbox rm "$name" 2>/dev/null || true
  done
}

@test "rm removes a specific sandbox" {
  create_test_sandbox "python" "$CLEANUP_WORKSPACE"
  local target="$SANDBOX_NAME"

  # Verify it exists
  run docker sandbox ls
  assert_output --partial "$target"

  # Remove it
  run cmd_rm "$target"
  assert_success

  # Verify it's gone
  run docker sandbox ls
  refute_output --partial "$target"
}

@test "rm all removes all sandboxes for workspace" {
  create_test_sandbox "python" "$CLEANUP_WORKSPACE"
  local sandbox1="$SANDBOX_NAME"
  sleep 1  # Ensure different timestamp for unique sandbox name
  create_test_sandbox "python" "$CLEANUP_WORKSPACE"
  local sandbox2="$SANDBOX_NAME"

  # Both should exist
  run docker sandbox ls
  assert_output --partial "$sandbox1"
  assert_output --partial "$sandbox2"

  # Remove all for this workspace
  cd "$CLEANUP_WORKSPACE"
  run cmd_rm "all"
  assert_success

  # Both should be gone
  run docker sandbox ls
  refute_output --partial "$sandbox1"
  refute_output --partial "$sandbox2"
}
```

- [ ] **Step 2: Run the test**

Run: `./tests/bats/bin/bats tests/integration/zz_cleanup.bats`

Expected: All 2 tests pass (or skip if docker sandbox not available).

- [ ] **Step 3: Commit**

```bash
git add tests/integration/zz_cleanup.bats
git commit -m "test: add integration tests for sandbox removal"
```

---

### Task 9: Final verification

- [ ] **Step 1: Run all unit tests**

Run: `make test`

Expected: All 39 unit tests pass.

- [ ] **Step 2: Run all integration tests**

Run: `make test-integration`

Expected: All 12 integration tests pass (or skip if docker sandbox not available).

- [ ] **Step 3: Run everything together**

Run: `make test-all`

Expected: All 39 unit tests and 12 integration tests pass.
