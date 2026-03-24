#!/usr/bin/env bats
# tests/ls.bats — unit tests for commands/ls.sh

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

  # Install default docker mock (logs invocations, exits 0)
  create_mock "docker"

  # Source the files under test
  # shellcheck source=../lib/helpers.sh
  source "${SCRIPT_DIR}/lib/helpers.sh"
  # shellcheck source=../commands/ls.sh
  source "${SCRIPT_DIR}/commands/ls.sh"
}

teardown() {
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Helper: create_mock
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
