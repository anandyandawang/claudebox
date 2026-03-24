# tests/test_helper/integration.bash
# Integration test infrastructure — real Docker sandbox operations

source "$(dirname "${BASH_SOURCE[0]}")/common.bash"

# Source helpers from production code
source "${SCRIPT_DIR}/src/lib/helpers.sh"

# Skip all tests if docker sandbox is not available.
# Call from setup_file() or individual tests.
require_docker_sandbox() {
  if ! command -v docker &>/dev/null; then
    skip "docker not found"
  fi
  if ! docker sandbox ls &>/dev/null 2>&1; then
    skip "docker sandbox not available — requires Docker Desktop with sandbox support"
  fi
}

# Build a template image (quiet mode).
# Usage: build_template_image "jvm"
build_template_image() {
  local template="$1"
  docker build -q -t "${template}-sandbox" "${SCRIPT_DIR}/templates/${template}"
}

# Create a test sandbox — replicates cmd_create steps 2-4 without running the sandbox.
# Requires image to be built first via build_template_image.
# Usage: create_test_sandbox "jvm" "/path/to/workspace"
# Sets: SANDBOX_NAME (exported)
create_test_sandbox() {
  local template="$1"
  local workspace="$2"
  local image_name="${template}-sandbox"
  local workspace_name
  workspace_name="$(printf '%s' "$(basename "${workspace}")" | tr -cs 'a-zA-Z0-9_.-' '-')"
  local session_id="sandbox-$(date +%Y%m%d-%H%M%S)"
  SANDBOX_NAME="${workspace_name}-${template}-${session_id}"
  export SANDBOX_NAME

  local host_claude_dir="${HOME}/.claude"
  docker sandbox create -t "${image_name}" --name "${SANDBOX_NAME}" claude "${workspace}" "${host_claude_dir}"

  # Symlink host config
  docker sandbox exec "${SANDBOX_NAME}" ln -sf "${host_claude_dir}/.claude.json" /home/agent/.claude.json
  docker sandbox exec "${SANDBOX_NAME}" ln -sf "${host_claude_dir}/settings.json" /home/agent/.claude/settings.json
  docker sandbox exec "${SANDBOX_NAME}" ln -sf "${host_claude_dir}/plugins" /home/agent/.claude/plugins

  # Copy workspace to container-local filesystem and create branch
  docker sandbox exec "${SANDBOX_NAME}" sh -c "
    cp -a '${workspace}/.' /home/agent/workspace/
    cd /home/agent/workspace
    git clean -fdx -q
    git checkout -b '${session_id}'
  "

  setup_environment "${SANDBOX_NAME}"
  wrap_claude_binary "${SANDBOX_NAME}"
}

# Apply network policy from a template's allowed-hosts.txt.
# Usage: apply_network_policy "jvm"
apply_network_policy() {
  local template="$1"
  local hosts_file="${SCRIPT_DIR}/templates/${template}/allowed-hosts.txt"
  if [[ -f "${hosts_file}" ]]; then
    local proxy_args=(--policy deny)
    while IFS= read -r host || [[ -n "$host" ]]; do
      [[ -z "$host" || "$host" == \#* ]] && continue
      proxy_args+=(--allow-host "$host")
    done < "${hosts_file}"
    docker sandbox network proxy "${SANDBOX_NAME}" "${proxy_args[@]}"
  fi
}

# Remove a test sandbox (silent on failure).
# Usage: cleanup_test_sandbox [name]
cleanup_test_sandbox() {
  local name="${1:-${SANDBOX_NAME:-}}"
  [[ -n "$name" ]] && docker sandbox rm "$name" 2>/dev/null || true
}

# Create a minimal git repo in a temporary directory.
# Usage: create_test_workspace "dirname"
# Returns: path via stdout
create_test_workspace() {
  local dirname="$1"
  local workspace="${BATS_FILE_TMPDIR}/${dirname}"
  mkdir -p "$workspace"
  git -C "$workspace" init -q
  echo "test content" > "$workspace/testfile.txt"
  git -C "$workspace" add .
  git -C "$workspace" commit -q -m "init"
  echo "$workspace"
}
