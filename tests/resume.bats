#!/usr/bin/env bats
# tests/resume.bats — unit tests for commands/resume.sh

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

  # Create a testproject workspace directory
  mkdir -p "${BATS_TEST_TMPDIR}/testproject"

  # Install docker mock with sandbox ls support (single sandbox by default)
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-1  running\n"
    fi
  '

  # Source the files under test (for direct calls in error-path tests)
  # shellcheck source=../lib/helpers.sh
  source "${SCRIPT_DIR}/lib/helpers.sh"
  # shellcheck source=../commands/resume.sh
  source "${SCRIPT_DIR}/commands/resume.sh"
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
# Error paths
# ---------------------------------------------------------------------------

@test "cmd_resume: errors when no sandboxes exist for workspace" {
  # Mock docker sandbox ls to return only the header (no sandbox rows)
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\n"
    fi
  '

  cd "${BATS_TEST_TMPDIR}/testproject"
  run cmd_resume

  assert_failure
  assert_output --partial "No sandboxes found"
}

@test "cmd_resume: errors on unknown arguments" {
  cd "${BATS_TEST_TMPDIR}/testproject"
  run cmd_resume --invalid

  assert_failure
  assert_output --partial "Unknown argument"
}

# ---------------------------------------------------------------------------
# Single sandbox
# ---------------------------------------------------------------------------

@test "cmd_resume: auto-selects single sandbox when user confirms" {
  run bash -c '
    source "${SCRIPT_DIR}/lib/helpers.sh"
    source "${SCRIPT_DIR}/commands/resume.sh"
    cd "${BATS_TEST_TMPDIR}/testproject"
    echo "Y" | cmd_resume
  '

  assert_success
  assert_output --partial "Resuming sandbox:"
  assert_output --partial "Starting sandbox"
}

@test "cmd_resume: exits cleanly when user declines single sandbox" {
  run bash -c '
    source "${SCRIPT_DIR}/lib/helpers.sh"
    source "${SCRIPT_DIR}/commands/resume.sh"
    cd "${BATS_TEST_TMPDIR}/testproject"
    echo "n" | cmd_resume
  '

  assert_success
  refute_output --partial "Resuming sandbox"
}

# ---------------------------------------------------------------------------
# Multiple sandboxes
# ---------------------------------------------------------------------------

@test "cmd_resume: picker selects correct sandbox from list" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$1 $2" == "sandbox ls" ]]; then
      printf "NAME STATUS\ntestproject-python-sandbox-1  running\ntestproject-jvm-sandbox-2  running\n"
    fi
  '

  run bash -c '
    source "${SCRIPT_DIR}/lib/helpers.sh"
    source "${SCRIPT_DIR}/commands/resume.sh"
    cd "${BATS_TEST_TMPDIR}/testproject"
    echo "2" | cmd_resume
  '

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

  run bash -c '
    source "${SCRIPT_DIR}/lib/helpers.sh"
    source "${SCRIPT_DIR}/commands/resume.sh"
    cd "${BATS_TEST_TMPDIR}/testproject"
    printf "99\n1\n" | cmd_resume
  '

  assert_success
  assert_output --partial "Invalid selection"
  assert_output --partial "Resuming sandbox: testproject-python-sandbox-1"
}

# ---------------------------------------------------------------------------
# Integration
# ---------------------------------------------------------------------------

@test "cmd_resume: calls setup_environment, refresh_credentials, wrap_claude_binary" {
  # Mock security to fail (no credentials) so refresh_credentials can complete
  create_mock "security" "" 1

  run bash -c '
    source "${SCRIPT_DIR}/lib/helpers.sh"
    source "${SCRIPT_DIR}/commands/resume.sh"
    cd "${BATS_TEST_TMPDIR}/testproject"
    echo "Y" | cmd_resume
  '

  assert_success
  assert_output --partial "Refreshing credentials"

  # Verify docker mock log has "sandbox exec" (from setup_environment and wrap_claude_binary)
  run grep "sandbox exec" "${MOCK_DOCKER_LOG}"
  assert_success
}

@test "cmd_resume: passes agent args after -- through to docker sandbox run" {
  create_mock "security" "" 1

  run bash -c '
    source "${SCRIPT_DIR}/lib/helpers.sh"
    source "${SCRIPT_DIR}/commands/resume.sh"
    cd "${BATS_TEST_TMPDIR}/testproject"
    echo "Y" | cmd_resume -- -p "fix the tests"
  '

  assert_success

  # Verify mock log has "sandbox run" with the agent args
  run grep "sandbox run" "${MOCK_DOCKER_LOG}"
  assert_success

  run grep "\-p" "${MOCK_DOCKER_LOG}"
  assert_success

  run grep "fix the tests" "${MOCK_DOCKER_LOG}"
  assert_success
}
