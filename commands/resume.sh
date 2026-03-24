# claudebox resume — resume an existing sandbox

cmd_resume() {
  # Parse agent args
  AGENT_ARGS=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --)
        shift
        AGENT_ARGS=("$@")
        break
        ;;
      *)
        echo "Unknown argument: $1" >&2
        echo "Usage: $(basename "$0") resume [-- agent_args...]" >&2
        return 1
        ;;
    esac
  done

  # List sandboxes for this workspace
  WORKSPACE_NAME="$(basename "$(pwd)" | tr -cs 'a-zA-Z0-9_.-' '-')"
  SANDBOXES=()
  while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    SANDBOXES+=("$name")
  done < <(docker sandbox ls 2>/dev/null | awk 'NR>1 {print $1}' | grep -E "^${WORKSPACE_NAME}-" || true)

  if [[ ${#SANDBOXES[@]} -eq 0 ]]; then
    echo "No sandboxes found for this workspace." >&2
    return 1
  fi

  if [[ ${#SANDBOXES[@]} -eq 1 ]]; then
    echo -n "Resume ${SANDBOXES[0]}? [Y/n]: "
    read -r CONFIRM || return 1
    if [[ "$CONFIRM" =~ ^[nN] ]]; then
      return 0
    fi
    SANDBOX_NAME="${SANDBOXES[0]}"
  else
    echo "Available sandboxes:"
    for i in "${!SANDBOXES[@]}"; do
      echo "  $((i + 1))) ${SANDBOXES[$i]}"
    done
    echo ""
    while true; do
      echo -n "Pick a sandbox [1-${#SANDBOXES[@]}]: "
      read -r PICK || return 1
      if [[ "$PICK" =~ ^[0-9]+$ ]] && (( PICK >= 1 && PICK <= ${#SANDBOXES[@]} )); then
        SANDBOX_NAME="${SANDBOXES[$((PICK - 1))]}"
        break
      fi
      echo "Invalid selection. Enter a number between 1 and ${#SANDBOXES[@]}."
    done
  fi

  echo "Resuming sandbox: ${SANDBOX_NAME}..."
  setup_environment "${SANDBOX_NAME}"
  refresh_credentials "${SANDBOX_NAME}"
  wrap_claude_binary "${SANDBOX_NAME}"

  echo "Starting sandbox..."
  docker sandbox run "${SANDBOX_NAME}" -- --dangerously-skip-permissions ${AGENT_ARGS[@]+"${AGENT_ARGS[@]}"}
}
