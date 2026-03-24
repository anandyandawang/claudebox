# tests/test_helper/common.bash
# Shared test infrastructure for claudebox bats tests

# Load bats helper libraries (vendored in test_helper/)
load "${BATS_TEST_DIRNAME}/test_helper/bats-support/load"
load "${BATS_TEST_DIRNAME}/test_helper/bats-assert/load"

# Resolve SCRIPT_DIR to the repo root (one level up from tests/)
SCRIPT_DIR="$(cd "$(dirname "${BATS_TEST_FILENAME}")/.." && pwd)"
export SCRIPT_DIR

setup() {
  # Create a temp directory for mock binaries and prepend to PATH
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"

  # Log file where mock docker records its invocations
  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  # Install the default docker mock
  create_mock "docker"
}

teardown() {
  # Clean up mock bin dir (BATS_TEST_TMPDIR is cleaned automatically)
  rm -rf "${MOCK_BIN_DIR}" 2>/dev/null || true
}

# Create a mock binary that logs calls and returns configurable output/exit code.
#
# Usage:
#   create_mock "docker"                          # default: logs args, returns 0
#   create_mock "docker" "some output" 0          # custom output and exit code
#   create_mock_script "docker" 'echo "custom"'   # fully custom script body
#
create_mock() {
  local name="$1"
  local output="${2:-}"
  local exit_code="${3:-0}"
  local log_var="MOCK_DOCKER_LOG"

  cat > "${MOCK_BIN_DIR}/${name}" <<MOCK
#!/usr/bin/env bash
echo "${name} \$*" >> "\${${log_var}}"
if [[ -n "${output}" ]]; then
  echo "${output}"
fi
exit ${exit_code}
MOCK
  chmod +x "${MOCK_BIN_DIR}/${name}"
}

# Create a mock with a fully custom script body.
# The script still has access to MOCK_DOCKER_LOG for logging.
create_mock_script() {
  local name="$1"
  local body="$2"

  cat > "${MOCK_BIN_DIR}/${name}" <<MOCK
#!/usr/bin/env bash
${body}
MOCK
  chmod +x "${MOCK_BIN_DIR}/${name}"
}
