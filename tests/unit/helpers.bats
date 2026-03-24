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

# ---------------------------------------------------------------------------
# Tests: refresh_credentials
# ---------------------------------------------------------------------------

@test "refresh_credentials: writes credentials to sandbox when keychain has them" {
  # Mock security to return a fake credential value
  create_mock "security" "supersecrettoken"

  run refresh_credentials "my-sandbox"

  assert_success
  # Should have made a docker sandbox exec call to write credentials
  assert [ -s "${MOCK_DOCKER_LOG}" ]
  run grep -q "docker sandbox exec my-sandbox" "${MOCK_DOCKER_LOG}"
  assert_success
}

@test "refresh_credentials: warns when no credentials found" {
  # Mock security to return exit code 1 (no credentials)
  create_mock "security" "" 1

  run refresh_credentials "my-sandbox"

  assert_success
  assert_output --partial "WARNING: No credentials found in Keychain"
}

# ---------------------------------------------------------------------------
# Tests: setup_environment
# ---------------------------------------------------------------------------

@test "setup_environment: truncates persistent env file" {
  run setup_environment "my-sandbox"

  assert_success
  # First docker call should be the truncate
  run bash -c "grep -m1 '' '${MOCK_DOCKER_LOG}'"
  assert_output --partial "truncate"
}

@test "setup_environment: exports GITHUB_USERNAME when set" {
  export GITHUB_USERNAME="testuser"

  run setup_environment "my-sandbox"

  assert_success
  run grep "GITHUB_USERNAME" "${MOCK_DOCKER_LOG}"
  assert_success

  unset GITHUB_USERNAME
}

@test "setup_environment: skips GITHUB_USERNAME when not set" {
  unset GITHUB_USERNAME

  run setup_environment "my-sandbox"

  assert_success
  run grep "GITHUB_USERNAME" "${MOCK_DOCKER_LOG}"
  # grep exits 1 when no match — GITHUB_USERNAME should NOT appear in docker log
  assert_failure
}

@test "setup_environment: makes at least 2 docker sandbox exec calls" {
  run setup_environment "my-sandbox"

  assert_success
  # Count lines in the docker log — each docker invocation is one line
  local call_count
  call_count="$(wc -l < "${MOCK_DOCKER_LOG}")"
  assert [ "${call_count}" -ge 2 ]
}

# ---------------------------------------------------------------------------
# Tests: wrap_claude_binary
# ---------------------------------------------------------------------------

@test "wrap_claude_binary: calls docker sandbox exec" {
  run wrap_claude_binary "my-sandbox"

  assert_success
  run grep "docker sandbox exec my-sandbox" "${MOCK_DOCKER_LOG}"
  assert_success
}

@test "wrap_claude_binary: script content includes claude-real guard" {
  run wrap_claude_binary "my-sandbox"

  assert_success
  # The inline script passed to sh -c is logged via $* in the docker mock.
  # Verify the claude-real idempotency guard appears in the logged invocation.
  run grep "claude-real" "${MOCK_DOCKER_LOG}"
  assert_success
}
