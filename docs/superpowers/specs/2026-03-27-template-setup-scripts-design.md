# Per-Template Setup Scripts

## Problem

`environment.Setup()` hardcodes JVM-specific logic (proxy configuration, CA cert import) that runs for every template. This is a layering violation — template-specific setup should live with the template.

Additionally, `environment.Setup()` truncates and re-writes `/etc/sandbox-persistent.sh` on every resume, but Docker sandbox filesystem persists across stop/resume cycles, so this is unnecessary.

## Design

### New convention: `setup.sh` per template

Each template directory can optionally include a `setup.sh` script. This follows the existing discovery pattern of `Dockerfile` and `allowed-hosts.txt`.

```
templates/jvm/
  Dockerfile
  allowed-hosts.txt
  setup.sh          <-- new
```

### `setup.sh` is read from the host template directory at create time

`sandbox.Manager` gets a `RunSetupScript(sandboxName, template)` method that checks if `templates/{template}/setup.sh` exists on the host, reads it, and executes it inside the container via `SandboxExec`. This follows the same pattern as `ApplyNetworkPolicy` reading `allowed-hosts.txt` from the template directory.

Since setup only runs at create time (not resume), the template name is always available — no need to bake the script into the image.

### `environment.Setup()` is removed

- **GITHUB_USERNAME export** moves into the generic create flow (written to `/etc/sandbox-persistent.sh` once at create time, directly in `create.go`).
- **JVM proxy/CA cert logic** moves into `templates/jvm/setup.sh`.
- **Truncation of `sandbox-persistent.sh`** is no longer needed — the file is written once at create time and persists.
- The `internal/environment/` package is deleted.

### Resume path simplifies

`resume.go` no longer calls `environment.Setup()`. It only does:
1. `credentials.Refresh()`
2. `mgr.WrapClaudeBinary()`
3. `mgr.Run()`

## Files changed

| File | Change |
|------|--------|
| `templates/jvm/setup.sh` | New — extracted JVM proxy/CA cert script |
| `internal/sandbox/sandbox.go` | Add `RunSetupScript(sandboxName, template)` — reads and executes `setup.sh` from template dir |
| `internal/commands/create.go` | Replace `environment.Setup()` with GITHUB_USERNAME export + `mgr.RunSetupScript()` |
| `internal/commands/resume.go` | Remove `environment.Setup()` call |
| `internal/environment/environment.go` | Delete |
| `internal/environment/environment_test.go` | Delete |

## `templates/jvm/setup.sh`

```bash
#!/bin/sh
# JVM proxy and CA cert configuration.
# Runs inside the sandbox at create time.

# Configure JVM proxy if HTTPS_PROXY is set (injected by Docker Desktop)
if [ -n "$HTTPS_PROXY" ]; then
  PROXY_HOST=$(echo "$HTTPS_PROXY" | sed -E "s|https?://||;s|:.*||")
  PROXY_PORT=$(echo "$HTTPS_PROXY" | sed -E "s|.*:([0-9]+).*|\1|")
  echo "export JAVA_TOOL_OPTIONS=\"-Dhttp.proxyHost=${PROXY_HOST} -Dhttp.proxyPort=${PROXY_PORT} -Dhttps.proxyHost=${PROXY_HOST} -Dhttps.proxyPort=${PROXY_PORT}\"" >> /etc/sandbox-persistent.sh
fi

# Import proxy CA cert into Java truststore
JAVA_HOME=$(java -XshowSettings:properties 2>&1 | grep "java.home" | awk "{print \$3}")
PROXY_CERT=$(find /usr/local/share/ca-certificates -name "*.crt" 2>/dev/null | head -1)
if [ -n "$PROXY_CERT" ] && [ -n "$JAVA_HOME" ]; then
  sudo keytool -import -trustcacerts -cacerts -storepass changeit -noprompt -alias proxy-ca -file "$PROXY_CERT" 2>/dev/null || true
fi
```

## Design decisions

- **`setup.sh` baked into image vs. read from host at create time:** Read from host. Since setup only runs at create time (not resume), the template name is always available. This avoids Dockerfile modifications and follows the same pattern as `allowed-hosts.txt`.
- **Run once at create vs. every resume:** Once at create. Docker sandbox filesystem persists, so re-running is unnecessary. Evidence: `WrapClaudeBinary` guards against double-wrapping, workspace copy only happens at create, symlinks only created at create.
- **Delete `environment` package vs. keep as thin wrapper:** Delete. After moving GITHUB_USERNAME inline and JVM logic to template, the package has no remaining responsibility.
