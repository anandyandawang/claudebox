#!/usr/bin/env bats
# tests/integration/filesystem.bats — integration tests for sandbox file layout

load "../test_helper/integration"

setup_file() {
  require_docker_sandbox

  INTTEST_WORKSPACE="$(create_test_workspace "cb-fs-$$")"
  export INTTEST_WORKSPACE

  build_template_image "jvm"
  create_test_sandbox "jvm" "$INTTEST_WORKSPACE"
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
