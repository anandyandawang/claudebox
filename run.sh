#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $(basename "$0") <template> [workspace] [-- agent_args...]"
  echo ""
  echo "  template     Template directory name (e.g. python, jvm)"
  echo "  workspace    Workspace path (default: current directory)"
  echo "  -- args      Additional arguments passed to the agent"
  echo ""
  echo "Available templates:"
  for dir in "$(dirname "$0")"/*/; do
    if [[ -f "${dir}Dockerfile" ]]; then
      echo "  $(basename "$dir")"
    fi
  done
  exit 1
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

[[ $# -lt 1 ]] && usage

TEMPLATE="$1"
shift

TEMPLATE_DIR="${SCRIPT_DIR}/${TEMPLATE}"

if [[ ! -f "${TEMPLATE_DIR}/Dockerfile" ]]; then
  echo "Error: No Dockerfile found in ${TEMPLATE_DIR}" >&2
  exit 1
fi

# Parse workspace and agent args
WORKSPACE="$(pwd)"
AGENT_ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --)
      shift
      AGENT_ARGS=("$@")
      break
      ;;
    *)
      WORKSPACE="$1"
      shift
      ;;
  esac
done

IMAGE_NAME="${TEMPLATE}-sandbox"
SANDBOX_NAME="${TEMPLATE}-$(basename "$WORKSPACE")"

# Step 1: Build the template image
echo "Building template image: ${IMAGE_NAME}..."
docker build -t "${IMAGE_NAME}" "${TEMPLATE_DIR}"

# Step 2: Create the sandbox (remove existing if present)
echo "Creating sandbox: ${SANDBOX_NAME}..."
if docker sandbox ls 2>/dev/null | grep -q "${SANDBOX_NAME}"; then
  echo "Sandbox ${SANDBOX_NAME} already exists, reusing it."
else
  HOST_CLAUDE_DIR="${HOME}/.claude"
  docker sandbox create -t "${IMAGE_NAME}" --name "${SANDBOX_NAME}" claude "${WORKSPACE}" "${HOST_CLAUDE_DIR}"

  # Step 3: Symlink host Claude config into the sandbox
  echo "Linking host Claude config..."
  docker sandbox exec "${SANDBOX_NAME}" rm -rf /home/agent/.claude/plugins
  docker sandbox exec "${SANDBOX_NAME}" ln -s "${HOST_CLAUDE_DIR}/plugins" /home/agent/.claude/plugins
  docker sandbox exec "${SANDBOX_NAME}" ln -sf "${HOST_CLAUDE_DIR}/settings.json" /home/agent/.claude/settings.json
  echo "Host config linked."

  # Step 4: Apply network policy if allowed-hosts.txt exists
  HOSTS_FILE="${TEMPLATE_DIR}/allowed-hosts.txt"
  if [[ -f "${HOSTS_FILE}" ]]; then
    echo "Applying network policy (deny by default)..."
    PROXY_ARGS=(--policy deny)
    while IFS= read -r host || [[ -n "$host" ]]; do
      # Skip empty lines and comments
      [[ -z "$host" || "$host" == \#* ]] && continue
      PROXY_ARGS+=(--allow-host "$host")
    done < "${HOSTS_FILE}"
    docker sandbox network proxy "${SANDBOX_NAME}" "${PROXY_ARGS[@]}"
    echo "Network policy applied: $(grep -cv '^\s*$\|^\s*#' "${HOSTS_FILE}") hosts allowed."
  else
    echo "No allowed-hosts.txt found, using default network policy (allow all)."
  fi
fi

# Run the sandbox
echo "Starting sandbox..."
docker sandbox run "${SANDBOX_NAME}" -- --dangerously-skip-permissions "${AGENT_ARGS[@]}"
