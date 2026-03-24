#!/usr/bin/env bats
# tests/unit/create.bats — unit tests for commands/create.sh

load "../test_helper/unit"

# ---------------------------------------------------------------------------
# Setup / Teardown
# ---------------------------------------------------------------------------

setup() {
  common_setup

  # Mock date to return a fixed value for deterministic sandbox names
  create_mock "date" "20260323-120000"

  # Mock security to fail (skip credential writing)
  create_mock "security" "" 1

  # Source the files under test
  # shellcheck source=../../src/lib/helpers.sh
  source "${SCRIPT_DIR}/src/lib/helpers.sh"
  # shellcheck source=../../src/commands/create.sh
  source "${SCRIPT_DIR}/src/commands/create.sh"
}

teardown() {
  common_teardown
}

# ---------------------------------------------------------------------------
# Helper: setup a fake template directory
# ---------------------------------------------------------------------------

# Creates a fake template dir under BATS_TEST_TMPDIR and overrides SCRIPT_DIR.
# Usage: setup_fake_template "mytemplate"
setup_fake_template() {
  local template_name="$1"
  local fake_root="${BATS_TEST_TMPDIR}/fake-script-dir"
  mkdir -p "${fake_root}/templates/${template_name}"
  touch "${fake_root}/templates/${template_name}/Dockerfile"
  export SCRIPT_DIR="${fake_root}"
}

# ---------------------------------------------------------------------------
# Test 1: Fails when template has no Dockerfile
# ---------------------------------------------------------------------------

@test "cmd_create: fails when template has no Dockerfile" {
  local fake_root="${BATS_TEST_TMPDIR}/empty-script-dir"
  mkdir -p "${fake_root}/templates/mytemplate"
  # No Dockerfile created
  export SCRIPT_DIR="${fake_root}"

  run cmd_create "mytemplate"

  assert_failure
  assert_output --partial "Error: No Dockerfile found"
}

# ---------------------------------------------------------------------------
# Test 2: Builds image and creates sandbox with correct names
# ---------------------------------------------------------------------------

@test "cmd_create: builds image and creates sandbox with correct names" {
  setup_fake_template "mytemplate"

  local workspace="${BATS_TEST_TMPDIR}/myworkspace"
  mkdir -p "${workspace}"

  run cmd_create "mytemplate" "${workspace}"

  assert_success
  # docker build -t mytemplate-sandbox should appear in the log
  run grep "docker build -t mytemplate-sandbox" "${MOCK_DOCKER_LOG}"
  assert_success

  # docker sandbox create -t mytemplate-sandbox --name should appear
  run grep "docker sandbox create -t mytemplate-sandbox --name" "${MOCK_DOCKER_LOG}"
  assert_success

  # docker sandbox run ... -- --dangerously-skip-permissions should appear
  run grep "docker sandbox run" "${MOCK_DOCKER_LOG}"
  assert_success
  run grep -- "--dangerously-skip-permissions" "${MOCK_DOCKER_LOG}"
  assert_success
}

# ---------------------------------------------------------------------------
# Test 3: Skips network policy when no allowed-hosts.txt
# ---------------------------------------------------------------------------

@test "cmd_create: skips network policy when no allowed-hosts.txt" {
  setup_fake_template "mytemplate"

  local workspace="${BATS_TEST_TMPDIR}/myworkspace"
  mkdir -p "${workspace}"

  run cmd_create "mytemplate" "${workspace}"

  assert_success
  assert_output --partial "No allowed-hosts.txt found"

  # Should NOT have called "sandbox network proxy"
  run grep "sandbox network proxy" "${MOCK_DOCKER_LOG}"
  assert_failure
}

# ---------------------------------------------------------------------------
# Test 4: Parses custom workspace argument
# ---------------------------------------------------------------------------

@test "cmd_create: parses custom workspace argument" {
  setup_fake_template "mytemplate"

  local workspace="${BATS_TEST_TMPDIR}/custom-workspace"
  mkdir -p "${workspace}"

  run cmd_create "mytemplate" "${workspace}"

  assert_success
  run grep "custom-workspace-mytemplate" "${MOCK_DOCKER_LOG}"
  assert_success
}

# ---------------------------------------------------------------------------
# Test 5: Passes agent args after -- to docker sandbox run
# ---------------------------------------------------------------------------

@test "cmd_create: passes agent args after -- to docker sandbox run" {
  setup_fake_template "mytemplate"

  local workspace="${BATS_TEST_TMPDIR}/myworkspace"
  mkdir -p "${workspace}"

  run cmd_create "mytemplate" "${workspace}" -- -p "fix the tests"

  assert_success

  run grep "sandbox run" "${MOCK_DOCKER_LOG}"
  assert_success

  run grep "\-p" "${MOCK_DOCKER_LOG}"
  assert_success

  run grep "fix the tests" "${MOCK_DOCKER_LOG}"
  assert_success
}

# ---------------------------------------------------------------------------
# Test 6: Symlinks host config and copies workspace
# ---------------------------------------------------------------------------

@test "cmd_create: symlinks host config and copies workspace" {
  setup_fake_template "mytemplate"

  local workspace="${BATS_TEST_TMPDIR}/myworkspace"
  mkdir -p "${workspace}"

  run cmd_create "mytemplate" "${workspace}"

  assert_success

  # Verify symlinks for host config
  run grep "ln -sf" "${MOCK_DOCKER_LOG}"
  assert_success

  run grep ".claude.json" "${MOCK_DOCKER_LOG}"
  assert_success

  run grep "settings.json" "${MOCK_DOCKER_LOG}"
  assert_success

  run grep "plugins" "${MOCK_DOCKER_LOG}"
  assert_success

  # Verify workspace copy
  run grep "cp -a" "${MOCK_DOCKER_LOG}"
  assert_success

  # Verify git branch creation
  run grep "git checkout -b" "${MOCK_DOCKER_LOG}"
  assert_success
}

# ---------------------------------------------------------------------------
# Test 7: Calls setup_environment, refresh_credentials, wrap_claude_binary
# ---------------------------------------------------------------------------

@test "cmd_create: calls setup_environment, refresh_credentials, wrap_claude_binary" {
  setup_fake_template "mytemplate"

  local workspace="${BATS_TEST_TMPDIR}/myworkspace"
  mkdir -p "${workspace}"

  run cmd_create "mytemplate" "${workspace}"

  assert_success

  # refresh_credentials prints this message
  assert_output --partial "Refreshing credentials"

  # setup_environment calls "sudo truncate" via docker sandbox exec
  run grep "truncate" "${MOCK_DOCKER_LOG}"
  assert_success

  # wrap_claude_binary references "claude-real" in the inline script
  run grep "claude-real" "${MOCK_DOCKER_LOG}"
  assert_success
}

# ---------------------------------------------------------------------------
# Test 8: Applies network policy when allowed-hosts.txt exists
# ---------------------------------------------------------------------------

@test "cmd_create: applies network policy when allowed-hosts.txt exists" {
  setup_fake_template "mytemplate"

  # Create allowed-hosts.txt with comments and 3 real hosts (no blank lines to avoid
  # macOS BRE grep -c quirk where \| alternation doesn't work and blank lines are counted)
  local template_dir="${BATS_TEST_TMPDIR}/fake-script-dir/templates/mytemplate"
  cat > "${template_dir}/allowed-hosts.txt" <<'EOF'
# This is a comment
# Another comment
pypi.org
api.github.com
files.pythonhosted.org
EOF

  local workspace="${BATS_TEST_TMPDIR}/myworkspace"
  mkdir -p "${workspace}"

  # Docker mock: exits 1 for curl to example.com (blocked), exits 0 for everything else
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    if [[ "$*" == *"curl"*"example.com"* ]]; then
      exit 1
    fi
  '

  run cmd_create "mytemplate" "${workspace}"

  assert_success
  assert_output --partial "Applying network policy"
  assert_output --partial "3 hosts allowed"
  assert_output --partial "Network policy verified"

  # Should have called "sandbox network proxy"
  run grep "sandbox network proxy" "${MOCK_DOCKER_LOG}"
  assert_success

  # Should have allowed the three hosts
  run grep -- "--allow-host pypi.org" "${MOCK_DOCKER_LOG}"
  assert_success

  run grep -- "--allow-host api.github.com" "${MOCK_DOCKER_LOG}"
  assert_success
}

# ---------------------------------------------------------------------------
# Test 9: Aborts when firewall verification fails (curl to example.com succeeds)
# ---------------------------------------------------------------------------

@test "cmd_create: aborts when firewall verification fails" {
  setup_fake_template "mytemplate"

  # Create allowed-hosts.txt so network policy path is taken
  local template_dir="${BATS_TEST_TMPDIR}/fake-script-dir/templates/mytemplate"
  cat > "${template_dir}/allowed-hosts.txt" <<'EOF'
pypi.org
api.github.com
EOF

  local workspace="${BATS_TEST_TMPDIR}/myworkspace"
  mkdir -p "${workspace}"

  # Docker mock: succeeds for ALL calls including curl example.com (firewall not working)
  create_mock_script "docker" '
    echo "docker $*" >> "${MOCK_DOCKER_LOG}"
    exit 0
  '

  run cmd_create "mytemplate" "${workspace}"

  assert_failure
  assert_output --partial "ERROR: Firewall verification failed"
}
