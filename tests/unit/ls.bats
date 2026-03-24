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

# ---------------------------------------------------------------------------
# Tests: cmd_ls
# ---------------------------------------------------------------------------

@test "cmd_ls: delegates to docker sandbox ls" {
  run cmd_ls

  assert_success
  # Verify the mock log shows the docker sandbox ls call
  run grep "docker sandbox ls" "${MOCK_DOCKER_LOG}"
  assert_success
}

@test "cmd_ls: succeeds with no sandboxes (empty output)" {
  # Mock docker sandbox ls to return empty output
  create_mock "docker"

  run cmd_ls

  assert_success
  assert_output ""
}
