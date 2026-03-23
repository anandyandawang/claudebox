# Design: `claudebox resume`

## Problem

`claudebox` creates a fresh sandbox on every invocation. There is no way to reconnect to an existing sandbox after a Claude session ends. Users lose their in-progress workspace state unless they manually extract changes before the session ends.

## Solution

Add a `claudebox resume` command that presents an interactive picker of existing sandboxes for the current workspace, re-runs idempotent setup steps, and launches a new Claude session inside the selected sandbox.

## CLI Interface

```
claudebox resume [-- agent_args...]
```

- No positional arguments. Always interactive.
- `-- agent_args` are passed through to `docker sandbox run`, same as the create path.

## Flow

1. Derive `WORKSPACE_NAME` from `$(basename "$(pwd)" | tr -cs 'a-zA-Z0-9_.-' '-')` (same as create and `rm all` paths).
2. Run `docker sandbox ls` and extract sandbox names from the first column, skipping the header row: `docker sandbox ls 2>/dev/null | awk 'NR>1 {print $1}'`.
3. Filter to sandboxes matching the current workspace using `grep -E "^${WORKSPACE_NAME}-"`.
4. If no sandboxes found, print "No sandboxes found for this workspace." and exit 1.
5. If one sandbox found, confirm with the user ("Resume <name>? [Y/n]").
6. If multiple found, display a numbered list and prompt for selection.
7. On the selected sandbox, re-run idempotent setup steps (see below).
8. Launch `docker sandbox run` with `--dangerously-skip-permissions` and any `agent_args`.

### Interactive Picker

```
$ claudebox resume
Available sandboxes:
  1) claudebox-python-sandbox-20260320-121500
  2) claudebox-jvm-sandbox-20260323-090000

Pick a sandbox [1-2]:
```

Invalid input re-prompts. Ctrl-C exits. EOF (Ctrl-D) exits cleanly (the `read` builtin returns non-zero on EOF, so use `read ... || exit 1` to handle `set -e`).

### Single Sandbox Shortcut

```
$ claudebox resume
Resume claudebox-python-sandbox-20260320-121500? [Y/n]:
```

Empty input or "y"/"Y" proceeds. "n"/"N" exits.

## Idempotent Setup Steps

These steps are safe to re-run and ensure the sandbox is in a working state even if ephemeral container state was partially lost:

1. **Refresh credentials** — Extract from macOS Keychain, write to `/home/agent/.claude/.credentials.json`.
2. **Re-set environment variables** — Truncate `/etc/sandbox-persistent.sh` before writing (not append) to avoid duplicate entries across multiple resumes. Then re-run the env var setup (GITHUB_USERNAME, JVM proxy config).
3. **Re-wrap claude binary** — Guard the `mv` with a check: only rename `claude` to `claude-real` if `claude-real` does not already exist. Without this guard, re-running `mv` on a resumed sandbox renames the wrapper script to `claude-real`, destroying the real binary and creating an infinite exec loop. The wrapper script itself is always rewritten.

```bash
CLAUDE_BIN=$(which claude)
if [ ! -f "${CLAUDE_BIN}-real" ]; then
  sudo mv "$CLAUDE_BIN" "${CLAUDE_BIN}-real"
fi
# Always rewrite the wrapper
sudo tee "$CLAUDE_BIN" > /dev/null << 'WRAPPER'
...
WRAPPER
sudo chmod +x "$CLAUDE_BIN"
```

### Steps NOT re-run on resume

- Image build (already built)
- Sandbox creation (already exists)
- Config symlinks (persist in container filesystem)
- Workspace copy (the whole point is to resume existing state)
- Git branch creation (already created)
- Network policy (persists across sessions)

## Script Changes

### Refactoring

Extract three functions from the current create flow so both create and resume can call them:

- `refresh_credentials(sandbox_name)` — Lines 197-205 of current script.
- `setup_environment(sandbox_name)` — Lines 141-161 (GITHUB_USERNAME + JVM proxy).
- `wrap_claude_binary(sandbox_name)` — Lines 210-219.

The create path calls these functions after sandbox creation. The resume path calls the same functions after sandbox selection.

### New Code

- Add `resume` case to the command dispatch block (after `ls` and `rm` handlers).
- Implement the interactive picker: parse `docker sandbox ls`, filter by workspace, display numbered list or single-sandbox confirmation.
- Parse `-- agent_args` for the resume command (same arg parsing as create).

### Usage Update

Update the `usage()` function to include:
```
  resume     Resume an existing sandbox (interactive picker)
```

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| No sandboxes exist | Print "No sandboxes found for this workspace." and exit 1 |
| Invalid picker input | Re-prompt until valid number entered |
| Ctrl-C / Ctrl-D during picker | Exit cleanly |
| `claude-real` already exists | Skip the `mv`, only rewrite the wrapper script |
| Sandbox currently running | Included in picker. If `docker sandbox run` fails on it, the error surfaces naturally. |
| Sandbox in broken state | Docker error surfaces naturally; user can `rm` and recreate |
| Credentials not in Keychain | Warning printed, continues without credentials (same as create) |
| Workspaces with shared name prefixes | Known limitation: `myapp-*` filter matches `myapp-v2-*` sandboxes too. Same behavior as existing `rm all`. |

## Testing

Manual testing:
1. Create a sandbox with `claudebox python`, make changes, exit.
2. Run `claudebox resume`, verify picker shows the sandbox.
3. Select it, verify credentials refresh, verify workspace state is preserved.
4. Test with multiple sandboxes from different templates.
5. Test edge cases: no sandboxes, single sandbox, invalid input.
