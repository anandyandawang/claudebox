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
  workspace="$(create_test_workspace "cb-create-$$")"

  build_template_image "python"
  create_test_sandbox "python" "$workspace"

  # Verify name matches: <workspace>-python-sandbox-YYYYMMDD-HHMMSS
  [[ "$SANDBOX_NAME" =~ ^cb-create-[0-9]+-python-sandbox-[0-9]{8}-[0-9]{6}$ ]]
}
