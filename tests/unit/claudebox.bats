#!/usr/bin/env bats
# tests/unit/claudebox.bats — unit tests for the top-level claudebox dispatcher

load "../test_helper/unit"

setup() {
  common_setup
  CLAUDEBOX="${SCRIPT_DIR}/claudebox"
}

teardown() {
  common_teardown
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

@test "claudebox: lists available templates (jvm) in usage" {
  run "${CLAUDEBOX}"

  assert_failure
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

@test "claudebox: routes unknown command to cmd_create (jvm template)" {
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

  run "${CLAUDEBOX}" jvm

  assert_success
  run grep "docker build -t jvm-sandbox" "${MOCK_DOCKER_LOG}"
  assert_success
}
