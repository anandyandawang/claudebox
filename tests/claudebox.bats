#!/usr/bin/env bats
# tests/claudebox.bats — unit tests for the top-level claudebox dispatcher

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

# ---------------------------------------------------------------------------
# Test 1: Shows usage and exits 1 with no args
# ---------------------------------------------------------------------------

@test "claudebox: shows usage and exits 1 with no args" {
  run "${CLAUDEBOX}"

  assert_failure
  assert_output --partial "Usage:"
  assert_output --partial "Available templates:"
}

# ---------------------------------------------------------------------------
# Test 2: Lists available templates in usage
# ---------------------------------------------------------------------------

@test "claudebox: lists available templates (python and jvm) in usage" {
  run "${CLAUDEBOX}"

  assert_failure
  assert_output --partial "python"
  assert_output --partial "jvm"
}

# ---------------------------------------------------------------------------
# Test 3: Routes ls to cmd_ls
# ---------------------------------------------------------------------------

@test "claudebox: routes ls to cmd_ls" {
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$*" == *"sandbox ls"* ]]; then
      echo "NAME STATUS"
      echo "test-sandbox  running"
    fi
  '

  run "${CLAUDEBOX}" ls

  assert_success
  assert_output --partial "test-sandbox"
  run grep "docker sandbox ls" "${MOCK_DOCKER_LOG}"
  assert_success
}

# ---------------------------------------------------------------------------
# Test 4: Routes rm to cmd_rm (no args → usage failure)
# ---------------------------------------------------------------------------

@test "claudebox: routes rm to cmd_rm (no args shows usage)" {
  run "${CLAUDEBOX}" rm

  assert_failure
  assert_output --partial "Usage:"
  assert_output --partial "rm"
}

# ---------------------------------------------------------------------------
# Test 5: Routes unknown command to cmd_create (template mode)
# ---------------------------------------------------------------------------

@test "claudebox: routes unknown command to cmd_create (python template)" {
  # Mock docker: log all calls; fail curl to example.com (firewall verification);
  # succeed for all other docker commands.
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$*" == *"curl"*"example.com"* ]]; then
      exit 1
    fi
  '

  # Mock date for deterministic SESSION_ID
  create_mock "date" "20260323-120000"

  # Mock security to fail (no credentials in keychain)
  create_mock "security" "" 1

  run "${CLAUDEBOX}" python

  assert_success
  run grep "docker build -t python-sandbox" "${MOCK_DOCKER_LOG}"
  assert_success
}
