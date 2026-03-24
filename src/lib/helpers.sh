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
