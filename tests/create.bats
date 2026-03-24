#!/usr/bin/env bats
# tests/create.bats — unit tests for commands/create.sh

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

  # Mock date to return a fixed value for deterministic sandbox names
  create_mock "date" "20260323-120000"

  # Mock security to fail (skip credential writing)
  create_mock "security" "" 1

  # Install default docker mock (logs invocations, exits 0)
  create_mock "docker"

  # Source the files under test
  # shellcheck source=../lib/helpers.sh
  source "${SCRIPT_DIR}/lib/helpers.sh"
  # shellcheck source=../commands/create.sh
  source "${SCRIPT_DIR}/commands/create.sh"
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
# Helper: setup a fake template directory
# ---------------------------------------------------------------------------

# Creates a fake template dir under BATS_TEST_TMPDIR and overrides SCRIPT_DIR.
# Usage: setup_fake_template "mytemplate"
setup_fake_template() {
  local template_name="$1"
  local fake_root="${BATS_TEST_TMPDIR}/fake-script-dir"
  mkdir -p "${fake_root}/${template_name}"
  touch "${fake_root}/${template_name}/Dockerfile"
  export SCRIPT_DIR="${fake_root}"
}

# ---------------------------------------------------------------------------
# Test 1: Fails when template has no Dockerfile
# ---------------------------------------------------------------------------

@test "cmd_create: fails when template has no Dockerfile" {
  local fake_root="${BATS_TEST_TMPDIR}/empty-script-dir"
  mkdir -p "${fake_root}/mytemplate"
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
  # WORKSPACE_NAME = basename("custom-workspace") | tr ... = "custom-workspace-"
  # SANDBOX_NAME = "custom-workspace--mytemplate-sandbox-20260323-120000"
  run grep "custom-workspace-" "${MOCK_DOCKER_LOG}"
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
  local template_dir="${BATS_TEST_TMPDIR}/fake-script-dir/mytemplate"
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
  local template_dir="${BATS_TEST_TMPDIR}/fake-script-dir/mytemplate"
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
