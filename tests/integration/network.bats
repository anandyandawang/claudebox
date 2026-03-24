#!/usr/bin/env bats
# tests/integration/network.bats — integration tests for network policy enforcement

load "../test_helper/integration"

setup_file() {
  require_docker_sandbox

  INTTEST_WORKSPACE="$(create_test_workspace "cb-net-$$")"
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
  local tmp_template="${BATS_FILE_TMPDIR}/nofilter-tpl"
  mkdir -p "$tmp_template"
  cp "${SCRIPT_DIR}/templates/python/Dockerfile" "$tmp_template/Dockerfile"

  # Build image from temporary template
  docker build -q -t "nofilter-sandbox" "$tmp_template"

  # Create a minimal sandbox (no full setup needed — just network access test)
  local nofilter_name="cb-nofilt-$$-sandbox-$(date +%Y%m%d-%H%M%S)"
  docker sandbox create -t "nofilter-sandbox" --name "$nofilter_name" claude "$INTTEST_WORKSPACE" "${HOME}/.claude"

  # No network policy applied — example.com should be reachable
  run docker sandbox exec "$nofilter_name" curl --connect-timeout 10 -sf https://example.com

  # Clean up before asserting (so sandbox and image are removed even on failure)
  docker sandbox rm "$nofilter_name" 2>/dev/null || true
  docker rmi "nofilter-sandbox" 2>/dev/null || true

  assert_success
}
