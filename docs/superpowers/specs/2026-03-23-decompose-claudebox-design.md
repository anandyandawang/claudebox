# Decompose `claudebox` into multiple files

## Problem

The `claudebox` script is 317 lines in a single file. It contains shared helper functions, a usage function, and four subcommand handlers all mixed together. This makes it harder to read and maintain as the tool grows.

## Design

Split the script into a thin dispatcher, a shared helpers library, and one file per subcommand.

### File structure

```
claudebox              # Entrypoint: SCRIPT_DIR resolution, usage(), dispatch
lib/
  helpers.sh           # Shared functions
commands/
  ls.sh                # cmd_ls()
  rm.sh                # cmd_rm()
  resume.sh            # cmd_resume()
  create.sh            # cmd_create()
```

### `claudebox` (entrypoint)

Responsibilities:
- Shebang, `set -euo pipefail`
- Resolve symlinks to find `SCRIPT_DIR` (lines 5-11 of current file)
- Source `lib/helpers.sh`
- Define `usage()` inline (it references `SCRIPT_DIR` to list templates)
- Check `$# -lt 1` and call `usage`
- Dispatch on `$1`: source the matching `commands/<cmd>.sh`, shift, call `cmd_<name>` with remaining args
- If `$1` doesn't match a known command, treat it as a template name and dispatch to `cmd_create`

### `lib/helpers.sh`

Contains three functions, unchanged from their current implementations:

- `refresh_credentials(sandbox_name)` — reads credentials from macOS Keychain and injects into sandbox
- `setup_environment(sandbox_name)` — configures persistent env vars and JVM proxy/certs in sandbox
- `wrap_claude_binary(sandbox_name)` — wraps the claude binary to cd to workspace before starting

### `commands/ls.sh`

```
cmd_ls() {
  docker sandbox ls 2>/dev/null
}
```

### `commands/rm.sh`

`cmd_rm()` — takes remaining args after `rm` is shifted off. Handles both `rm all` (scoped to current workspace) and `rm <name>`.

### `commands/resume.sh`

`cmd_resume()` — takes remaining args after `resume` is shifted off. Parses `-- agent_args...`, lists sandboxes for current workspace, interactive picker, then calls `setup_environment`, `refresh_credentials`, `wrap_claude_binary`, and `docker sandbox run`.

### `commands/create.sh`

`cmd_create()` — takes `$1` as template name, remaining args as workspace/agent args. Handles:
1. Template validation
2. Arg parsing (workspace, `-- agent_args...`)
3. Image build
4. Sandbox creation
5. Host config linking
6. Workspace copy
7. Network policy application and verification
8. Credential refresh
9. Binary wrapping
10. `docker sandbox run`

### Shared state

- `SCRIPT_DIR` is set in the entrypoint before any command file is sourced, so it's available to all command functions and `usage()` as a global variable.
- Helper functions are sourced once in the entrypoint, so all command files can call them directly.

### Behavioral changes

None. The decomposition is purely structural. All commands, arguments, output, and behavior remain identical.
