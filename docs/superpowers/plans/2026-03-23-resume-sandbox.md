# `claudebox resume` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `claudebox resume` command that lets users reconnect to existing sandboxes via an interactive picker.

**Architecture:** Refactor the `claudebox` bash script to extract three reusable functions from the create path (`refresh_credentials`, `setup_environment`, `wrap_claude_binary`), then add a `resume` subcommand that uses them. The resume command lists existing sandboxes for the current workspace, presents an interactive picker, re-runs idempotent setup, and launches a Claude session.

**Tech Stack:** Bash, Docker Sandbox API

**Spec:** `docs/superpowers/specs/2026-03-23-resume-sandbox-design.md`

---

### Task 1: Extract `refresh_credentials` function

**Files:**
- Modify: `claudebox:196-205`

This extracts lines 196-205 into a function and replaces the original code with a function call. The function takes `SANDBOX_NAME` as its argument.

- [ ] **Step 1: Add the function definition after line 11 (after SCRIPT_DIR)**

Add this function after the `SCRIPT_DIR` line and before the `usage()` function:

```bash
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
```

- [ ] **Step 2: Replace lines 196-205 with a function call**

Replace the inline credential refresh code (the block starting with `# Refresh credentials from macOS Keychain on every launch`) with:

```bash
refresh_credentials "${SANDBOX_NAME}"
```

- [ ] **Step 3: Verify no syntax errors**

Run: `bash -n claudebox`
Expected: No output (clean parse)

- [ ] **Step 4: Commit**

```bash
git add claudebox
git commit -m "refactor: extract refresh_credentials function"
```

---

### Task 2: Extract `setup_environment` function

**Files:**
- Modify: `claudebox:140-161` (after Task 1, line numbers will have shifted — find by the comment `# Step 5: Set GitHub username`)

This extracts the GITHUB_USERNAME and JVM proxy setup into a function. Per the spec, this function truncates `/etc/sandbox-persistent.sh` before writing to avoid duplicate entries on resume.

- [ ] **Step 1: Add the function definition after `refresh_credentials`**

```bash
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
```

- [ ] **Step 2: Replace the inline code with a function call**

Replace the block from `# Step 5: Set GitHub username` through the end of the JVM proxy `sh -c '...'` block (lines ending with the closing `'`) with:

```bash
  setup_environment "${SANDBOX_NAME}"
```

Note: This is inside the `{ ... }` block of the create path, so keep the indentation with 2 spaces.

- [ ] **Step 3: Verify no syntax errors**

Run: `bash -n claudebox`
Expected: No output (clean parse)

- [ ] **Step 4: Commit**

```bash
git add claudebox
git commit -m "refactor: extract setup_environment function"
```

---

### Task 3: Extract `wrap_claude_binary` function with idempotency guard

**Files:**
- Modify: `claudebox:207-219` (approximate — find by the comment `# Wrap the claude binary`)

This extracts the claude binary wrapper into a function and adds the guard to prevent the infinite exec loop on resume (the critical bug from the spec review).

- [ ] **Step 1: Add the function definition after `setup_environment`**

```bash
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

- [ ] **Step 2: Replace the inline code with a function call**

Replace the block from `# Wrap the claude binary` through the closing `'` with:

```bash
wrap_claude_binary "${SANDBOX_NAME}"
```

- [ ] **Step 3: Verify no syntax errors**

Run: `bash -n claudebox`
Expected: No output (clean parse)

- [ ] **Step 4: Commit**

```bash
git add claudebox
git commit -m "refactor: extract wrap_claude_binary function with idempotency guard"
```

---

### Task 4: Update `usage()` and add `resume` subcommand

**Files:**
- Modify: `claudebox` — `usage()` function and command dispatch block

This is the core new feature: the `resume` subcommand with the interactive picker.

- [ ] **Step 1: Update `usage()` to include resume**

In the `usage()` function, after the line `echo "  rm all       Remove all sandboxes for the current directory"`, add:

```bash
  echo "  resume       Resume an existing sandbox (interactive picker)"
```

Also update the first two lines of usage to include resume:

```bash
  echo "Usage: $(basename "$0") <template> [workspace] [-- agent_args...]"
  echo "       $(basename "$0") resume [-- agent_args...]"
  echo "       $(basename "$0") rm <sandbox-name|all>"
```

- [ ] **Step 2: Add the `resume` subcommand handler**

After the `rm` handler block (after its `exit 0` / `fi`), and before the `TEMPLATE="$1"` line, add:

```bash
if [[ "$1" == "resume" ]]; then
  shift
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
        exit 1
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
    exit 1
  fi

  if [[ ${#SANDBOXES[@]} -eq 1 ]]; then
    echo -n "Resume ${SANDBOXES[0]}? [Y/n]: "
    read -r CONFIRM || exit 1
    if [[ "$CONFIRM" =~ ^[nN] ]]; then
      exit 0
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
      read -r PICK || exit 1
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
  exit 0
fi
```

- [ ] **Step 3: Verify no syntax errors**

Run: `bash -n claudebox`
Expected: No output (clean parse)

- [ ] **Step 4: Commit**

```bash
git add claudebox
git commit -m "feat: add claudebox resume command with interactive picker"
```

---

### Task 5: Manual testing

- [ ] **Step 1: Verify `claudebox` usage output shows resume**

Run: `./claudebox`
Expected: Usage output includes `resume` in the commands list.

- [ ] **Step 2: Verify `claudebox resume` with no sandboxes**

Run: `./claudebox resume` (from a directory with no existing sandboxes)
Expected: "No sandboxes found for this workspace." and exit code 1.

- [ ] **Step 3: End-to-end test with a real sandbox**

1. Create a sandbox: `./claudebox python`
2. Make a change inside the sandbox (e.g., create a file), then exit.
3. Run `./claudebox resume` — verify the picker shows the sandbox.
4. Select it — verify credentials refresh, wrapper setup, and Claude launches.
5. Verify the file created in step 2 still exists (workspace state preserved).

- [ ] **Step 4: Test multi-sandbox picker**

1. Create two sandboxes with different templates.
2. Run `./claudebox resume` — verify both appear in the numbered list.
3. Pick one — verify it resumes correctly.

- [ ] **Step 5: Test invalid input handling**

1. Run `./claudebox resume` with multiple sandboxes.
2. Enter invalid input (0, 99, "abc", empty) — verify re-prompt.
3. Enter valid number — verify it proceeds.
