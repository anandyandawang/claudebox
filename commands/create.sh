# claudebox create — create a new sandbox from a template

cmd_create() {
  TEMPLATE="$1"
  shift

  TEMPLATE_DIR="${SCRIPT_DIR}/${TEMPLATE}"

  if [[ ! -f "${TEMPLATE_DIR}/Dockerfile" ]]; then
    echo "Error: No Dockerfile found in ${TEMPLATE_DIR}" >&2
    return 1
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

  # Step 1: Build the template image
  echo "Building template image: ${IMAGE_NAME}..."
  docker build -t "${IMAGE_NAME}" "${TEMPLATE_DIR}"

  # Step 2: Create the sandbox, mounting the repo for the initial copy
  WORKSPACE_NAME="$(printf '%s' "$(basename "${WORKSPACE}")" | tr -cs 'a-zA-Z0-9_.-' '-')"
  SESSION_ID="sandbox-$(date +%Y%m%d-%H%M%S)"
  SANDBOX_NAME="${WORKSPACE_NAME}-${TEMPLATE}-${SESSION_ID}"

  echo "Creating sandbox: ${SANDBOX_NAME}..."
  {
    HOST_CLAUDE_DIR="${HOME}/.claude"
    docker sandbox create -t "${IMAGE_NAME}" --name "${SANDBOX_NAME}" claude "${WORKSPACE}" "${HOST_CLAUDE_DIR}"

    # Step 3: Symlink host config into the sandbox
    echo "Linking host Claude config..."
    docker sandbox exec "${SANDBOX_NAME}" ln -sf "${HOST_CLAUDE_DIR}/.claude.json" /home/agent/.claude.json
    docker sandbox exec "${SANDBOX_NAME}" ln -sf "${HOST_CLAUDE_DIR}/settings.json" /home/agent/.claude/settings.json
    docker sandbox exec "${SANDBOX_NAME}" ln -sf "${HOST_CLAUDE_DIR}/plugins" /home/agent/.claude/plugins
    echo "Host config linked."

    # Step 4: Copy the repo to the container's local filesystem.
    # Docker's VirtioFS mounts on macOS have write-visibility latency that corrupts
    # Gradle's fileHashes.bin and build output jars mid-build. A local copy avoids
    # this entirely — all file I/O and git operations happen on local ext4.
    # The mount is only used as the source for the initial copy.
    echo "Copying workspace to container-local filesystem..."
    docker sandbox exec "${SANDBOX_NAME}" sh -c "
      cp -a '${WORKSPACE}/.' /home/agent/workspace/
      cd /home/agent/workspace
      git clean -fdx -q
      git checkout -b '${SESSION_ID}'
    "
    setup_environment "${SANDBOX_NAME}"

    # Step 5: Apply network policy if allowed-hosts.txt exists
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

      # Verify network policy
      echo "Verifying network policy..."
      if docker sandbox exec "${SANDBOX_NAME}" curl --connect-timeout 5 -sf https://example.com >/dev/null 2>&1; then
        echo "ERROR: Firewall verification failed — was able to reach https://example.com" >&2
        return 1
      else
        echo "  Blocked:  https://example.com (as expected)"
      fi
      if docker sandbox exec "${SANDBOX_NAME}" curl --connect-timeout 5 -sf https://api.github.com/zen >/dev/null 2>&1; then
        echo "  Allowed:  https://api.github.com (as expected)"
      else
        echo "ERROR: Firewall verification failed — unable to reach https://api.github.com" >&2
        return 1
      fi
      echo "Network policy verified."
    else
      echo "No allowed-hosts.txt found, using default network policy (allow all)."
    fi
  }

  refresh_credentials "${SANDBOX_NAME}"

  wrap_claude_binary "${SANDBOX_NAME}"

  # Run the sandbox
  echo "Starting sandbox..."
  docker sandbox run "${SANDBOX_NAME}" -- --dangerously-skip-permissions ${AGENT_ARGS[@]+"${AGENT_ARGS[@]}"}
}
