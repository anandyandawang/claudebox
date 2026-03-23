# Decompose claudebox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the monolithic `claudebox` bash script into a thin dispatcher, a shared helpers library, and one file per subcommand.

**Architecture:** The entrypoint (`claudebox`) resolves `SCRIPT_DIR`, sources `lib/helpers.sh`, defines `usage()`, then dispatches to `commands/<cmd>.sh` files. Each command file defines a single `cmd_<name>()` function. `exit` becomes `return` in extracted functions.

**Tech Stack:** Bash

**Spec:** `docs/superpowers/specs/2026-03-23-decompose-claudebox-design.md`

---

### Task 1: Create `lib/helpers.sh`

**Files:**
- Create: `lib/helpers.sh`

- [ ] **Step 1: Create the file**

Create `lib/helpers.sh` with the three shared functions extracted verbatim from `claudebox` lines 13-78. No shebang, no `set` directives.

```bash
# Shared helper functions for claudebox commands

refresh_credentials() {
  local sandbox_name="$1"
  echo "Refreshing credentials from macOS Keychain..."
  local creds
  creds="$(security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null)" || {
    echo "WARNING: No credentials found in Keychain. You may need to re-authenticate inside the sandbox." >&2
    creds=""
  }
  if [[ -n "$creds" ]]; then
    local creds_b64
    creds_b64="$(printf '%s' "$creds" | base64)"
    docker sandbox exec "${sandbox_name}" sh -c "echo '${creds_b64}' | tr -d '[:space:]' | base64 -d > /home/agent/.claude/.credentials.json && chmod 600 /home/agent/.claude/.credentials.json"
  fi
}

setup_environment() {
  local sandbox_name="$1"
  # Truncate persistent env file to avoid duplicates on resume
  docker sandbox exec "${sandbox_name}" sh -c "sudo truncate -s 0 /etc/sandbox-persistent.sh"

  # Set GitHub username (GITHUB_TOKEN is auto-injected by `docker sandbox run`)
  if [[ -n "${GITHUB_USERNAME:-}" ]]; then
    docker sandbox exec "${sandbox_name}" sh -c "echo 'export GITHUB_USERNAME=${GITHUB_USERNAME}' >> /etc/sandbox-persistent.sh"
  fi

  # Configure JVM proxy so Java respects the sandbox HTTP proxy.
  # The sandbox MITM proxy intercepts HTTPS with its own CA cert. Without importing it
  # into Java's truststore, Gradle/Maven fail to download dependencies through the proxy,
  # causing errors that look like cache corruption.
  docker sandbox exec "${sandbox_name}" sh -c '
    if [ -n "$HTTPS_PROXY" ]; then
      PROXY_HOST=$(echo "$HTTPS_PROXY" | sed -E "s|https?://||;s|:.*||")
      PROXY_PORT=$(echo "$HTTPS_PROXY" | sed -E "s|.*:([0-9]+).*|\1|")
      echo "export JAVA_TOOL_OPTIONS=\"-Dhttp.proxyHost=${PROXY_HOST} -Dhttp.proxyPort=${PROXY_PORT} -Dhttps.proxyHost=${PROXY_HOST} -Dhttps.proxyPort=${PROXY_PORT}\"" >> /etc/sandbox-persistent.sh
    fi

    JAVA_HOME=$(java -XshowSettings:properties 2>&1 | grep "java.home" | awk "{print \$3}")
    PROXY_CERT=$(find /usr/local/share/ca-certificates -name "*.crt" 2>/dev/null | head -1)
    if [ -n "$PROXY_CERT" ] && [ -n "$JAVA_HOME" ]; then
      sudo keytool -import -trustcacerts -cacerts -storepass changeit -noprompt -alias proxy-ca -file "$PROXY_CERT" 2>/dev/null || true
    fi
  '
}

wrap_claude_binary() {
  local sandbox_name="$1"
  # Wrap the claude binary so it cd's to the local workspace before starting.
  # docker sandbox run always starts in the mounted workspace, but we need Claude Code's
  # project directory to be the local copy so Edit/Read/Glob and Bash all use the same files.
  #
  # Guard: only move the original binary if claude-real doesn't already exist.
  # Without this, resuming a sandbox would rename the wrapper to claude-real,
  # destroying the real binary and creating an infinite exec loop.
  docker sandbox exec "${sandbox_name}" sh -c '
    CLAUDE_BIN=$(which claude)
    if [ ! -f "${CLAUDE_BIN}-real" ]; then
      sudo mv "$CLAUDE_BIN" "${CLAUDE_BIN}-real"
    fi
    sudo tee "$CLAUDE_BIN" > /dev/null << WRAPPER
#!/bin/bash
cd /home/agent/workspace
exec "\$(dirname "\$0")/claude-real" "\$@"
WRAPPER
    sudo chmod +x "$CLAUDE_BIN"
  '
}
```

- [ ] **Step 2: Syntax check**

Run: `bash -n lib/helpers.sh`
Expected: No output (clean parse)

- [ ] **Step 3: Commit**

```bash
git add lib/helpers.sh
git commit -m "refactor: extract shared helpers to lib/helpers.sh"
```

---

### Task 2: Create `commands/ls.sh`

**Files:**
- Create: `commands/ls.sh`

- [ ] **Step 1: Create the file**

```bash
# claudebox ls — list all sandboxes

cmd_ls() {
  docker sandbox ls 2>/dev/null
}
```

- [ ] **Step 2: Syntax check**

Run: `bash -n commands/ls.sh`
Expected: No output

- [ ] **Step 3: Commit**

```bash
git add commands/ls.sh
git commit -m "refactor: extract ls command to commands/ls.sh"
```

---

### Task 3: Create `commands/rm.sh`

**Files:**
- Create: `commands/rm.sh`

- [ ] **Step 1: Create the file**

Extract from `claudebox` lines 115-144. Convert `exit` to `return`.

```bash
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
```

- [ ] **Step 2: Syntax check**

Run: `bash -n commands/rm.sh`
Expected: No output

- [ ] **Step 3: Commit**

```bash
git add commands/rm.sh
git commit -m "refactor: extract rm command to commands/rm.sh"
```

---

### Task 4: Create `commands/resume.sh`

**Files:**
- Create: `commands/resume.sh`

- [ ] **Step 1: Create the file**

Extract from `claudebox` lines 147-211. Convert `exit` to `return`.

```bash
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
```

- [ ] **Step 2: Syntax check**

Run: `bash -n commands/resume.sh`
Expected: No output

- [ ] **Step 3: Commit**

```bash
git add commands/resume.sh
git commit -m "refactor: extract resume command to commands/resume.sh"
```

---

### Task 5: Create `commands/create.sh`

**Files:**
- Create: `commands/create.sh`

- [ ] **Step 1: Create the file**

Extract from `claudebox` lines 213-317. Convert `exit` to `return`. Preserve the brace-group structure for steps 4-7.

```bash
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
  WORKSPACE_NAME="$(basename "${WORKSPACE}" | tr -cs 'a-zA-Z0-9_.-' '-')"
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
```

- [ ] **Step 2: Syntax check**

Run: `bash -n commands/create.sh`
Expected: No output

- [ ] **Step 3: Commit**

```bash
git add commands/create.sh
git commit -m "refactor: extract create command to commands/create.sh"
```

---

### Task 6: Rewrite `claudebox` as thin dispatcher

**Files:**
- Modify: `claudebox`

- [ ] **Step 1: Rewrite the entrypoint**

Replace the entire file with the dispatcher. It sources helpers, defines `usage()`, and routes to command files.

```bash
#!/usr/bin/env bash
set -euo pipefail

# Resolve symlinks to find the real script location
SOURCE="${BASH_SOURCE[0]}"
while [[ -L "$SOURCE" ]]; do
  DIR="$(cd "$(dirname "$SOURCE")" && pwd)"
  SOURCE="$(readlink "$SOURCE")"
  [[ "$SOURCE" != /* ]] && SOURCE="$DIR/$SOURCE"
done
SCRIPT_DIR="$(cd "$(dirname "$SOURCE")" && pwd)"

source "${SCRIPT_DIR}/lib/helpers.sh"

usage() {
  echo "Usage: $(basename "$0") <template> [workspace] [-- agent_args...]"
  echo "       $(basename "$0") resume [-- agent_args...]"
  echo "       $(basename "$0") rm <sandbox-name|all>"
  echo ""
  echo "  template     Template directory name (e.g. python, jvm)"
  echo "  workspace    Workspace path (default: current directory)"
  echo "  -- args      Additional arguments passed to the agent"
  echo ""
  echo "Each run creates a new sandbox with a local copy of the repo,"
  echo "so multiple sessions can work on independent branches in parallel."
  echo ""
  echo "Commands:"
  echo "  ls           List all sandboxes"
  echo "  rm <name>    Remove a sandbox"
  echo "  rm all       Remove all sandboxes for the current directory"
  echo "  resume       Resume an existing sandbox (interactive picker)"
  echo ""
  echo "Available templates:"
  for dir in "${SCRIPT_DIR}"/*/; do
    if [[ -f "${dir}Dockerfile" ]]; then
      echo "  $(basename "$dir")"
    fi
  done
  exit 1
}

[[ $# -lt 1 ]] && usage

case "$1" in
  ls)
    source "${SCRIPT_DIR}/commands/ls.sh"
    shift
    cmd_ls "$@"
    ;;
  rm)
    source "${SCRIPT_DIR}/commands/rm.sh"
    shift
    cmd_rm "$@"
    ;;
  resume)
    source "${SCRIPT_DIR}/commands/resume.sh"
    shift
    cmd_resume "$@"
    ;;
  *)
    source "${SCRIPT_DIR}/commands/create.sh"
    cmd_create "$@"
    ;;
esac
```

- [ ] **Step 2: Syntax check**

Run: `bash -n claudebox`
Expected: No output

- [ ] **Step 3: Verify usage output**

Run: `./claudebox`
Expected: Usage text with available templates, identical to before

- [ ] **Step 4: Commit**

```bash
git add claudebox
git commit -m "refactor: rewrite claudebox as thin dispatcher

Sources lib/helpers.sh and dispatches to commands/*.sh files.
No behavioral changes — all commands work identically."
```
