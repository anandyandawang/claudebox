# Sandbox Branch Naming: Use Sandbox ID

**Date:** 2026-04-01
**Status:** Accepted

## Problem

Sandbox branch names (`sandbox-YYYYMMDD-HHMMSS`) are timestamp-based and completely disconnected from the sandbox container name. This makes it hard to correlate a branch with its sandbox. The sandbox naming system already has a unique, human-friendly ID component (`MMDD-cat-hash`, e.g., `0401-chonk-f3`) that would serve better as the branch name.

## Decision

Replace `GenerateSessionID()` with `GenerateSandboxID(template)`, which returns the sandbox instance ID (`MMDD-cat-hash`). Use this ID as both the git branch name inside the sandbox and as a component of the sandbox container name.

## Changes

### `internal/sandbox/naming.go`

1. **Add `GenerateSandboxID(template string) string`** — returns `MMDD-cat-hash` (e.g., `0401-chonk-f3`). Extracts the cat name, instance hash, and date formatting currently inlined in `GenerateSandboxName`.

2. **Change `GenerateSandboxName` signature** from `(workspacePath, template string)` to `(workspacePath, sandboxID string)`. The function becomes a simple composition: `WorkspacePrefix(workspacePath) + sandboxID`.

3. **Delete `GenerateSessionID()`** — no longer needed.

### `internal/commands/create.go`

4. Generate the sandbox ID first, then pass it to both the sandbox name and the session ID:
   ```go
   sandboxID := sandbox.GenerateSandboxID(template)
   sandboxName := sandbox.GenerateSandboxName(workspace, sandboxID)
   // ...
   SessionID: sandboxID,
   ```

### `tests/integration/helpers_test.go`

5. Update `createTestSandbox` to use `GenerateSandboxID` instead of `GenerateSessionID`, and pass the sandbox ID to `GenerateSandboxName`.

### `internal/sandbox/naming_test.go`

6. Replace `TestGenerateSessionID` with `TestGenerateSandboxID` — regex changes from `^sandbox-\d{8}-\d{6}$` to `^\d{4}-[a-z]{5}-[0-9a-f]{2}$`.

7. Update `TestGenerateSandboxName` and all tests calling `GenerateSandboxName` to pass a sandbox ID string instead of a template string.

### `tests/integration/filesystem_test.go`

8. Update branch pattern check from `strings.HasPrefix(branch, "sandbox-")` to a regex matching `\d{4}-[a-z]{5}-[0-9a-f]{2}`.

## Files Unchanged

- `internal/commands/rm.go` and `internal/commands/resume.go` use `WorkspacePrefix` only — unaffected.
- `internal/sandbox/sandbox.go` uses `opts.SessionID` opaquely — no change needed.

## Branch Name Format

| Before | After |
|--------|-------|
| `sandbox-20260401-192408` | `0401-chonk-f3` |

The new format is shorter, human-friendly, and directly ties the branch to its sandbox container.
