# Sandbox Naming Redesign

## Problem

Docker Desktop creates a Unix socket at `~/.docker/sandboxes/vm/<sandbox-name>/docker-public.sock`. macOS limits Unix socket paths to 104 bytes (103 usable chars). Long workspace directory names cause the full path to exceed this limit, resulting in `bind: invalid argument` errors.

The current naming format `<workspace>-<template>-sandbox-YYYYMMDD-HHMMSS` is unbounded and does not account for this constraint.

## Design

### Name format

```
<wshash(2)>-<workspace(12)>.<MMDD>-<cat(5)>-<hash(2)>
```

The name has two parts separated by a dot:

- **Workspace identity** (left of dot): identifies which project this sandbox belongs to
  - **WS hash**: first 2 hex chars of SHA-256 of the full (untruncated) workspace path. Differentiates workspaces that truncate to the same prefix.
  - **Workspace**: first 12 chars of sanitized workspace directory name, trailing hyphens/dots stripped after truncation.

- **Sandbox instance ID** (right of dot): identifies this particular sandbox run
  - **MMDD**: month and day of creation.
  - **Cat name**: random meme cat name, max 5 chars, from a hardcoded list.
  - **Hash**: first 2 hex chars of SHA-256 of the full (untruncated) template name + cat name + Unix microsecond timestamp. Ensures uniqueness without querying existing sandboxes.

### Max length

Worst case: `2 + 1 + 12 + 1 + 4 + 1 + 5 + 1 + 2 = 29 chars`

Socket path budget: `103 - 22 (Docker prefix after home) - 29 (sandbox name) - 20 (socket suffix) = 32 chars for home directory`

`/Users/christopherjohnson` (25 chars) fits with 7 to spare.

### Example

```
Full workspace:       lambda-jpm-clearings
Full template:        jvm
Date:                 March 25
Cat name:             chonk
Unix microseconds:    1711382400000000

WS hash input:        "lambda-jpm-clearings"
WS hash:              sha256("lambda-jpm-clearings")[:2] → b4

Truncated workspace:  lambda-jpm-c

Final hash input:     "jvm" + "chonk" + "1711382400000000"
Final hash:           sha256("jvm" + "chonk" + "1711382400000000")[:2] → f3

Name:   b4-lambda-jpm-c.0325-chonk-f3
        ^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^
        workspace part   sandbox id
```

### Prefix matching for `rm all`

`rm all` uses the workspace identity (left of dot + dot) as the prefix:

```
b4-lambda-jpm-c.
```

Because the ws hash is derived from the full untruncated workspace name, two workspaces that truncate to the same 12 chars (e.g., `lambda-jpm-clearings` vs `lambda-jpm-clients`) get different prefixes and won't collide on `rm all`.

### Uniqueness

Two separate hashes cover different concerns:

- **WS hash** (stable per workspace): ensures workspace identity is preserved despite truncation. Two workspaces with the same 12-char prefix get different ws hashes.
- **Final hash** (unique per creation): computed over full template + cat name + Unix microsecond timestamp. Ensures each sandbox creation produces a unique name without querying existing sandboxes.

The only collision scenario is: same full workspace + same template + same cat name + same microsecond. Physically impossible since sandbox creation is a blocking operation.

### Cat name list

Hardcoded list of ~25 meme cat names, max 5 chars each:

```go
var catNames = []string{
    "nyan", "maru", "chonk", "floof", "blep",
    "mlem", "loaf", "beans", "bongo", "mochi",
    "luna", "simba", "felix", "salem", "tom",
    "tux", "void", "smol", "purr", "meow",
    "socks", "fluff", "grump", "chomp", "boop",
}
```

### User-facing output

On sandbox creation, print the name so users can reference it later:

```
Sandbox created: b4-lambda-jpm-c.0325-chonk-f3
```

The same name appears in `claudebox ls` and is used with `claudebox rm` and `claudebox resume`.

## Changes

### `internal/sandbox/naming.go`

- `GenerateSandboxName` updated to produce the new format
- Function signature unchanged: `GenerateSandboxName(workspacePath, template string) string`
- New helper: `truncateClean(s string, max int) string` — truncates and strips trailing hyphens/dots
- New helper: `randomCatName() string` — picks a random cat name from the hardcoded list
- New helper: `workspaceHash(fullWorkspace string) string` — 2-char hex hash of full workspace
- New helper: `instanceHash(template, cat string) string` — 2-char hex hash of template + cat + microsecond timestamp

### `internal/commands/create.go`

- `GenerateSessionID()` is still used for the git branch name inside the sandbox (passed as `CreateOpts.SessionID`). It stays as-is — only the sandbox name changes.
- The `fmt.Printf("Creating sandbox: %s...\n", sandboxName)` line already prints the name.

### `internal/commands/rm.go`

- Update prefix for `rm all` to use `<wshash>-<truncated-workspace>.` instead of `<workspace>-`.

### `internal/sandbox/naming_test.go`

- Cover: truncation, trailing-char stripping, hash determinism, prefix generation for `rm all`.
