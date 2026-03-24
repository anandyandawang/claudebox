# Repo Restructuring Design

## Problem

Tool source code (`commands/`, `lib/`), templates (`jvm/`, `python/`), and tests (`tests/`) all sit at the repository root as peers. This makes it hard to tell what is "the tool" vs "template data" vs "test infrastructure."

## Solution

Move tool source into `src/` and templates into `templates/`. Keep `claudebox` at the root as the user-facing entry point. No functional changes.

## Directory Layout

```
claudebox                  # entry point (stays at root)
src/
  commands/
    create.sh
    ls.sh
    resume.sh
    rm.sh
  lib/
    helpers.sh
templates/
  jvm/
    Dockerfile
    allowed-hosts.txt
  python/
    Dockerfile
    allowed-hosts.txt
tests/                     # stays at root
Makefile                   # stays at root
README.md                  # stays at root
docs/                      # stays at root
```

## Code Changes

### 1. `claudebox` dispatcher

Update `SCRIPT_DIR`-relative paths:

- `source "$SCRIPT_DIR/lib/helpers.sh"` becomes `source "$SCRIPT_DIR/src/lib/helpers.sh"`
- `source "$SCRIPT_DIR/commands/<cmd>.sh"` becomes `source "$SCRIPT_DIR/src/commands/<cmd>.sh"`
- Template directory resolution: templates are now at `$SCRIPT_DIR/templates/`

### 2. `src/commands/create.sh`

Update template path resolution from `$SCRIPT_DIR/<template>` to `$SCRIPT_DIR/templates/<template>`.

### 3. Tests

Update source paths in all `.bats` files to reference `src/commands/` and `src/lib/` instead of `commands/` and `lib/`.

### 4. README

Update any file path references to reflect the new layout.

## Scope

- No functional changes
- No new features
- Same behavior, reorganized files
