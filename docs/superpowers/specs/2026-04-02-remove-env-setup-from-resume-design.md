# Remove environment.Setup() from Resume Flow

**Date:** 2026-04-02
**Status:** Approved

## Summary

Environment variables written to `/etc/sandbox-persistent.sh` during the `create` flow persist across sandbox stop/start cycles. The `environment.Setup()` call in the `resume` flow is therefore redundant — it truncates and re-writes the same values every time. Remove it.

## Motivation

- `environment.Setup()` during resume is unnecessary work: it truncates `/etc/sandbox-persistent.sh`, re-exports `GITHUB_USERNAME`, and re-runs JVM proxy/keytool configuration — all of which already persist from the create flow.
- Host-side env vars (`GITHUB_USERNAME`, `HTTPS_PROXY`) do not realistically change between sessions, so refreshing them on resume adds no value.
- Removing the call makes resume faster and simpler.

## Design

### Changes

1. **`internal/commands/resume.go`**: Remove the `environment.Setup(d, sandboxName)` call and its comment. Remove the `environment` import.

2. **`internal/commands/commands_test.go`**: Remove assertions for environment.Setup-related docker exec calls in resume tests (truncate and GITHUB_USERNAME export commands).

### Additional cleanup

- `internal/environment/environment.go` — removed truncation of `/etc/sandbox-persistent.sh` (dead code now that Setup only runs on create).
- `internal/environment/environment_test.go` — removed `TestSetupTruncatesPersistentEnv` (tested removed behavior).

### No changes

- `internal/commands/create.go` — unchanged, still calls `environment.Setup()`.

### Test changes

- `internal/commands/commands_test.go` — added `TestResumeWrapBinaryFailure` for WrapClaudeBinary error propagation on resume.
- `tests/integration/filesystem_test.go` — added re-wrap integration tests (auto-update replacement and full binary replacement).

### What stays in resume

- `mgr.RefreshConfig()` — re-syncs settings.json and plugins from host (config changes between sessions).
- `credentials.Refresh()` — re-loads credentials from macOS Keychain (credentials expire).
- `mgr.Run()` — starts the sandbox.

### WrapClaudeBinary stays in resume

- `mgr.WrapClaudeBinary()` must run on resume. The wrapper script persists across stop/start, but Claude Code auto-updates can replace the binary at the `claude` path, overwriting the wrapper with a fresh binary. Without re-wrapping on resume, the sandbox boots in the empty mount directory instead of the workspace. A `CLAUDEBOX_WRAPPER` sentinel in the wrapper script detects whether `claude` is still the wrapper or has been replaced by an auto-update. If replaced, the new binary is moved to `claude-real` before re-writing the wrapper — ensuring the sandbox picks up the updated binary.

## Side Benefits

- Resume is faster: no longer runs truncate, GITHUB_USERNAME export, or JVM proxy/keytool commands.
- Simpler resume flow with fewer failure points.
