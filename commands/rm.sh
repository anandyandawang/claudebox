# claudebox rm — remove sandboxes

cmd_rm() {
  [[ $# -lt 1 ]] && { echo "Usage: $(basename "$0") rm <sandbox-name|all>" >&2; return 1; }

  if [[ "$1" == "all" ]]; then
    WORKSPACE_NAME="$(basename "$(pwd)" | tr -cs 'a-zA-Z0-9_.-' '-')"
    SANDBOX_LIST="$(docker sandbox ls 2>/dev/null || true)"
    REMOVED=0
    while IFS= read -r SANDBOX_NAME; do
      [[ -z "$SANDBOX_NAME" ]] && continue
      docker sandbox rm "${SANDBOX_NAME}" && echo "Removed sandbox: ${SANDBOX_NAME}" || true
      REMOVED=$((REMOVED + 1))
    done < <(echo "$SANDBOX_LIST" | grep -oE "^${WORKSPACE_NAME}-[^ ]+" || true)
    if [[ $REMOVED -eq 0 ]]; then
      echo "No sandboxes found for $(pwd)."
    else
      echo "Removed ${REMOVED} sandbox(es)."
    fi
    return 0
  fi

  SANDBOX_NAME="$1"
  if docker sandbox ls 2>/dev/null | grep -q "${SANDBOX_NAME}"; then
    docker sandbox rm "${SANDBOX_NAME}"
    echo "Removed sandbox: ${SANDBOX_NAME}"
  else
    echo "Sandbox ${SANDBOX_NAME} not found." >&2
    return 1
  fi
}
