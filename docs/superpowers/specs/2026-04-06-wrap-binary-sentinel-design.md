# Fix WrapClaudeBinary to Detect Auto-Update Replacements

**Date:** 2026-04-06
**Status:** Approved

## Summary

`WrapClaudeBinary` uses an idempotent guard (`if [ ! -f claude-real ]`) to avoid re-moving the binary on repeated calls. When Claude Code auto-updates, it replaces only the `claude` binary — not our `claude-real` backup. The guard sees `claude-real` exists and skips the `mv`, so the new binary at `claude` is overwritten by the wrapper and the sandbox continues running the old `claude-real`. The auto-update is silently discarded.

Fix: embed a `CLAUDEBOX_WRAPPER` sentinel comment in the wrapper script and check for it. If `claude` lacks the sentinel, it's been replaced by an auto-update — move it to `claude-real` before re-writing the wrapper.

## Bug Reproduction

1. Create sandbox: `claude` (v1) moved to `claude-real`, wrapper written to `claude`.
2. Claude Code auto-updates: replaces `claude` with new binary v2. `claude-real` remains v1.
3. Resume calls `WrapClaudeBinary`:
   - `claude-real` exists (v1) -> guard skips `mv`
   - `tee` overwrites new v2 binary with wrapper
   - Wrapper exec's `claude-real` -> runs old v1
4. New binary v2 is silently lost.

## Design

### New wrapper script

```bash
CLAUDE_BIN=$(which claude)
if [ ! -f "${CLAUDE_BIN}-real" ]; then
  sudo mv "$CLAUDE_BIN" "${CLAUDE_BIN}-real"
elif ! grep -q 'CLAUDEBOX_WRAPPER' "$CLAUDE_BIN"; then
  sudo mv "$CLAUDE_BIN" "${CLAUDE_BIN}-real"
fi
sudo tee "$CLAUDE_BIN" > /dev/null << 'WRAPPER'
#!/bin/bash
# CLAUDEBOX_WRAPPER
cd /home/agent/workspace
exec "$(dirname "$0")/claude-real" "$@"
WRAPPER
sudo chmod +x "$CLAUDE_BIN"
```

Three-way logic:
1. **No `claude-real`**: First wrap or full package replacement. `mv` triggers.
2. **`claude-real` exists, `claude` lacks sentinel**: Auto-update replaced wrapper with new binary. `mv` triggers — new binary becomes `claude-real`.
3. **`claude-real` exists, `claude` has sentinel**: Wrapper still intact. No `mv` — only re-writes wrapper (idempotent).

### Changes

1. **`internal/sandbox/sandbox.go`** — Update the script string in `WrapClaudeBinary` to include the sentinel comment in the wrapper and the `grep -q` detection in the guard.

2. **`tests/integration/filesystem_test.go`**:
   - **"re-wrap after binary replacement restores wrapper"**: Fix assertion — after re-wrap, `claude-real` should now contain the new fake binary (not the old one). The sentinel guard detects the replacement and moves the new binary to `claude-real`.
   - **Add "re-wrap is idempotent when wrapper intact"**: Call `WrapClaudeBinary` twice with no auto-update in between. Verify `claude-real` is unchanged — the sentinel prevents unnecessary `mv`.

3. **`internal/sandbox/sandbox_test.go`** — Update unit test if it asserts on the script content string.

4. **`docs/superpowers/specs/2026-04-02-remove-env-setup-from-resume-design.md`** — Update the "WrapClaudeBinary stays in resume" section to reflect sentinel-based detection.

### No changes

- `internal/commands/create.go` — calls `WrapClaudeBinary`, unchanged.
- `internal/commands/resume.go` — calls `WrapClaudeBinary`, unchanged.
- `internal/docker/docker.go` — execution interface unchanged.
- Network policy, credentials, config refresh — unrelated.
