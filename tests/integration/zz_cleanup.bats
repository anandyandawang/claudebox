#!/usr/bin/env bats
# tests/integration/zz_cleanup.bats — integration tests for sandbox removal
# Named zz_ to run last, after other test files have cleaned up.

load "../test_helper/integration"

# Source rm.sh for cmd_rm
source "${SCRIPT_DIR}/src/commands/rm.sh"

setup_file() {
  require_docker_sandbox

  CLEANUP_WORKSPACE="$(create_test_workspace "cb-cleanup-$$")"
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
