# tests/test_helper/unit.bash
# Unit test infrastructure — mock binaries and shared setup/teardown

source "$(dirname "${BASH_SOURCE[0]}")/common.bash"

# Base setup for unit tests: mock bin directory, docker call log, default docker mock.
# Call from each test file's setup() function.
common_setup() {
  MOCK_BIN_DIR="$(mktemp -d "${BATS_TEST_TMPDIR}/mock-bin.XXXXXX")"
  export MOCK_BIN_DIR
  export PATH="${MOCK_BIN_DIR}:${PATH}"

  MOCK_DOCKER_LOG="${BATS_TEST_TMPDIR}/docker-calls.log"
  export MOCK_DOCKER_LOG
  : > "${MOCK_DOCKER_LOG}"

  create_mock "docker"
}

# Base teardown for unit tests.
common_teardown() {
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
