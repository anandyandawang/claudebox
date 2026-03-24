#!/usr/bin/env bats
# tests/rm.bats — unit tests for commands/rm.sh

load "test_helper/bats-support/load"
load "test_helper/bats-assert/load"

# ---------------------------------------------------------------------------
# Setup / Teardown
# ---------------------------------------------------------------------------

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

  # Install default docker mock (logs invocations, exits 0)
  create_mock "docker"

  # Source the files under test
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  source "${SCRIPT_DIR}/src/commands/rm.sh"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Helper: create_mock / create_mock_script
# ---------------------------------------------------------------------------

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

create_mock_script() {
  local name="$1"
  local body="$2"

  cat > "${MOCK_BIN_DIR}/${name}" <<MOCK
#!/usr/bin/env bash
${body}
MOCK
  chmod +x "${MOCK_BIN_DIR}/${name}"
}

# ---------------------------------------------------------------------------
# Basic tests
# ---------------------------------------------------------------------------

@test "cmd_rm: shows usage when called with no args" {
  run cmd_rm

  assert_failure
  assert_output --partial "Usage:"
  assert_output --partial "rm"
}

@test "cmd_rm: removes a named sandbox that exists" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "mysandbox  running"
    fi
  '

  run cmd_rm "mysandbox"

  assert_success
  assert_output --partial "Removed sandbox:"
}

@test "cmd_rm: prints error when sandbox not found" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "othersandbox  running"
    fi
  '

  run cmd_rm "mysandbox"

  assert_failure
  assert_output --partial "not found"
}

@test "cmd_rm: partial name match with grep -q (substring matching)" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "foobar-sandbox  running"
    fi
  '

  run cmd_rm "foo"

  assert_success
  assert_output --partial "Removed sandbox:"
}

@test "cmd_rm: docker sandbox rm failure — still prints Removed (no set -e)" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "mysandbox  running"
    fi
    if [[ "$1 $2" == "sandbox rm" ]]; then
      exit 1
    fi
  '

  run cmd_rm "mysandbox"

  assert_success
  assert_output --partial "Removed sandbox:"
}

# ---------------------------------------------------------------------------
# rm all tests
# ---------------------------------------------------------------------------

@test "cmd_rm all: removes only sandboxes matching current workspace name" {
  local workdir="${BATS_TEST_TMPDIR}/myproject"
  mkdir -p "$workdir"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "myproject-sandbox1  running\n"
      printf "myproject-sandbox2  running\n"
      printf "otherproject-sandbox  running\n"
    fi
  '

  cd "$workdir"
  run cmd_rm "all"

  assert_success
  assert_output --partial "Removed 2 sandbox(es)"
  # Non-matching sandbox should not have been removed
  run grep "sandbox rm.*otherproject" "${MOCK_DOCKER_LOG}"
  assert_failure
}

@test "cmd_rm all: prints message when no sandboxes found" {
  local workdir="${BATS_TEST_TMPDIR}/myproject2"
  mkdir -p "$workdir"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      echo "otherproject-sandbox  running"
    fi
  '

  cd "$workdir"
  run cmd_rm "all"

  assert_success
  assert_output --partial "No sandboxes found"
}

@test "cmd_rm all: counts attempted removals including failures" {
  local workdir="${BATS_TEST_TMPDIR}/myproject3"
  mkdir -p "$workdir"

  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "myproject3-sandbox1  running\n"
      printf "myproject3-sandbox2  running\n"
    fi
    if [[ "$1 $2" == "sandbox rm" ]]; then
      exit 1
    fi
  '

  cd "$workdir"
  run cmd_rm "all"

  assert_success
  assert_output --partial "Removed 2 sandbox(es)"
}
