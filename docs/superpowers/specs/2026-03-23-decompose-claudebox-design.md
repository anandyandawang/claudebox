# Decompose `claudebox` into multiple files

## Problem

The `claudebox` script is 316 lines in a single file. It contains shared helper functions, a usage function, and four subcommand handlers all mixed together. This makes it harder to read and maintain as the tool grows.

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

### Conventions

- Command and library files are sourced, not executed directly. They must not contain shebangs or `set` directives.
- Each command file defines a single `cmd_<name>()` function.
- `exit` statements in the original code become `return` statements in extracted functions (the dispatcher exits naturally after the function returns).
- The `${AGENT_ARGS[@]+"${AGENT_ARGS[@]}"}` expansion pattern must be preserved — it guards against `set -u` errors when the array is empty.

### `claudebox` (entrypoint)

Responsibilities:
- Shebang, `set -euo pipefail`
- Resolve symlinks to find `SCRIPT_DIR` (lines 5-11 of current file)
- Source `lib/helpers.sh`
- Define `usage()` inline (it references `SCRIPT_DIR` to list templates)
- Check `$# -lt 1` and call `usage`
- Dispatch on `$1`:
  - For named subcommands (`ls`, `rm`, `resume`): shift `$1` off, source `commands/<cmd>.sh`, call `cmd_<name> "$@"`
  - For anything else (template name): source `commands/create.sh`, call `cmd_create "$@"` **without shifting** — `$1` is the template name that `cmd_create` needs

### `lib/helpers.sh`

Contains three functions, unchanged from their current implementations:

- `refresh_credentials(sandbox_name)` — reads credentials from macOS Keychain and injects into sandbox
- `setup_environment(sandbox_name)` — configures persistent env vars and JVM proxy/certs in sandbox
- `wrap_claude_binary(sandbox_name)` — wraps the claude binary to cd to workspace before starting

### `commands/ls.sh`

```bash
cmd_ls() {
  docker sandbox ls 2>/dev/null
}
```

### `commands/rm.sh`

`cmd_rm()` — direct extraction of lines 116-144 from the original file. Takes remaining args after `rm` is shifted off by the dispatcher.

- Validates `$# -lt 1` and prints inline usage (`Usage: claudebox rm <sandbox-name|all>`) on failure
- `rm all`: computes `WORKSPACE_NAME` from `pwd`, iterates matching sandboxes via `grep -oE`, counts and reports removals
- `rm <name>`: validates sandbox exists via `docker sandbox ls | grep`, then removes

### `commands/resume.sh`

`cmd_resume()` — direct extraction of lines 148-211 from the original file. Takes remaining args after `resume` is shifted off by the dispatcher.

- Parses `-- agent_args...` from remaining args
- Computes `WORKSPACE_NAME` from `pwd`, lists matching sandboxes
- Single sandbox: confirmation prompt; multiple: numbered interactive picker
- Calls `setup_environment`, `refresh_credentials`, `wrap_claude_binary`
- Runs `docker sandbox run` with `--dangerously-skip-permissions` and any agent args

### `commands/create.sh`

`cmd_create()` — direct extraction of lines 213-317 from the original file. Receives all args (template name is `$1`).

Steps 1-3 (template validation, arg parsing, image build) run first. Steps 4-7 are grouped in a brace block `{ ... }` — this preserves the original structure where sandbox creation, config linking, workspace copy, and network policy are grouped together. Steps 8-10 (credential refresh, binary wrapping, `docker sandbox run`) run after the brace block, matching the original flow.

1. Template validation (`TEMPLATE_DIR="${SCRIPT_DIR}/${TEMPLATE}"`, check for Dockerfile)
2. Arg parsing (workspace path, `-- agent_args...`)
3. Image build (`docker build -t "${TEMPLATE}-sandbox"`)
4. Sandbox creation (`docker sandbox create`)
5. Host config linking (symlinks for `.claude.json`, `settings.json`, `plugins`)
6. Workspace copy (`cp -a`, `git clean`, `git checkout -b`)
7. Network policy (read `allowed-hosts.txt`, apply deny policy with allowed hosts, verify blocked/allowed)
8. Credential refresh
9. Binary wrapping
10. `docker sandbox run` with `--dangerously-skip-permissions`

### Shared state

- `SCRIPT_DIR` is set in the entrypoint before any command file is sourced, so it's available to all command functions and `usage()` as a global variable. `commands/create.sh` uses it to resolve `TEMPLATE_DIR`.
- Helper functions are sourced once in the entrypoint, so all command files can call them directly.

### Behavioral changes

None. The decomposition is purely structural. All commands, arguments, output, and behavior remain identical.
