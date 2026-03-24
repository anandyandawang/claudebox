# tests/test_helper/common.bash
# Shared test infrastructure — bats libraries and repo root resolution

# Resolve SCRIPT_DIR to the repo root (two levels up from tests/{unit,integration}/)
SCRIPT_DIR="$(cd "$(dirname "${BATS_TEST_FILENAME}")/../.." && pwd)"
export SCRIPT_DIR

# Load bats helper libraries via absolute paths
load "${SCRIPT_DIR}/tests/test_helper/bats-support/load"
load "${SCRIPT_DIR}/tests/test_helper/bats-assert/load"
