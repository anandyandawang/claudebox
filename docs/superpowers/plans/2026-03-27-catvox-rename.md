# Catvox Rename Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the project from "claudebox" to "catvox" across the entire codebase — module, binary, imports, CLI strings, docs.

**Architecture:** Mechanical rename with no behavioral changes. Go module path changes from `claudebox` to `catvox`, directory `cmd/claudebox` moves to `cmd/catvox`, all import paths update accordingly. README gets a new section explaining the name origin.

**Tech Stack:** Go 1.21+, Cobra CLI

---

### Task 1: Rename Go module and update all import paths

**Files:**
- Modify: `go.mod:1` — module name
- Modify: `cmd/claudebox/main.go:5` — import path
- Modify: `internal/commands/create.go:5-8` — import paths
- Modify: `internal/commands/resume.go:6-9` — import paths
- Modify: `internal/commands/rm.go:5-6` — import paths
- Modify: `internal/commands/ls.go:5` — import path
- Modify: `internal/commands/commands_test.go:6-7` — import paths
- Modify: `internal/commands/resume_test.go:5` — import path
- Modify: `internal/sandbox/sandbox.go:6` — import path
- Modify: `internal/sandbox/sandbox_test.go:5` — import path
- Modify: `internal/environment/environment.go:4` — import path
- Modify: `internal/environment/environment_test.go:4` — import path
- Modify: `internal/credentials/keychain.go:4` — import path
- Modify: `internal/credentials/keychain_test.go:4` — import path
- Modify: `tests/integration/helpers_test.go:6-8` — import paths

- [ ] **Step 1: Update module name in go.mod**

In `go.mod`, change line 1:

```
module claudebox
```

to:

```
module catvox
```

- [ ] **Step 2: Update all import paths in source files**

In every `.go` file listed above, replace all occurrences of `"claudebox/internal/` with `"catvox/internal/`. The affected imports:

`cmd/claudebox/main.go`:
```go
"catvox/internal/commands"
"catvox/internal/docker"
```

`internal/commands/create.go`:
```go
"catvox/internal/credentials"
"catvox/internal/docker"
"catvox/internal/environment"
"catvox/internal/sandbox"
```

`internal/commands/resume.go`:
```go
"catvox/internal/credentials"
"catvox/internal/docker"
"catvox/internal/environment"
"catvox/internal/sandbox"
```

`internal/commands/rm.go`:
```go
"catvox/internal/docker"
"catvox/internal/sandbox"
```

`internal/commands/ls.go`:
```go
"catvox/internal/docker"
```

`internal/commands/commands_test.go`:
```go
"catvox/internal/docker"
"catvox/internal/sandbox"
```

`internal/commands/resume_test.go`:
```go
"catvox/internal/docker"
```

`internal/sandbox/sandbox.go`:
```go
"catvox/internal/docker"
```

`internal/sandbox/sandbox_test.go`:
```go
"catvox/internal/docker"
```

`internal/environment/environment.go`:
```go
"catvox/internal/docker"
```

`internal/environment/environment_test.go`:
```go
"catvox/internal/docker"
```

`internal/credentials/keychain.go`:
```go
"catvox/internal/docker"
```

`internal/credentials/keychain_test.go`:
```go
"catvox/internal/docker"
```

`tests/integration/helpers_test.go`:
```go
"catvox/internal/docker"
"catvox/internal/environment"
"catvox/internal/sandbox"
```

- [ ] **Step 3: Verify the module compiles**

Run: `go build ./...`
Expected: clean build, no errors

- [ ] **Step 4: Commit**

```bash
git add go.mod cmd/claudebox/main.go internal/ tests/
git commit -m "refactor: rename Go module from claudebox to catvox"
```

---

### Task 2: Rename cmd directory and update build config

**Files:**
- Rename: `cmd/claudebox/` → `cmd/catvox/`
- Modify: `Makefile:4,17` — binary name and build path
- Modify: `.gitignore:33` — binary entry

- [ ] **Step 1: Rename the cmd directory**

```bash
mv cmd/claudebox cmd/catvox
```

- [ ] **Step 2: Update the comment at the top of main.go**

In `cmd/catvox/main.go`, change line 1:

```go
// cmd/claudebox/main.go
```

to:

```go
// cmd/catvox/main.go
```

- [ ] **Step 3: Update Makefile**

Replace the full `Makefile` contents with:

```makefile
.PHONY: build test test-unit test-integration test-all clean

build:
	go build -o catvox ./cmd/catvox

test: test-unit

test-unit:
	go test ./...

test-integration:
	go test -tags integration -v ./tests/integration/ -timeout 300s

test-all: test-unit test-integration

clean:
	rm -f catvox
```

- [ ] **Step 4: Update .gitignore**

Change the last line of `.gitignore` from:

```
claudebox
```

to:

```
catvox
```

- [ ] **Step 5: Verify build produces the correct binary**

```bash
make build && ls -la catvox
```

Expected: `catvox` binary exists in project root.

- [ ] **Step 6: Run unit tests**

Run: `make test`
Expected: all tests pass

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: rename cmd directory and build config to catvox"
```

---

### Task 3: Update CLI strings in main.go

**Files:**
- Modify: `cmd/catvox/main.go:23-30` — Cobra command Use and Long strings

- [ ] **Step 1: Update the Cobra root command strings**

In `cmd/catvox/main.go`, replace the root command definition:

```go
rootCmd := &cobra.Command{
    Use:   "claudebox [template] [workspace] [-- agent_args...]",
    Short: "Run Claude Code in sandboxed Docker containers",
    Long: `claudebox creates isolated Docker sandbox environments for Claude Code
with per-template toolchains and network restrictions.

Each run creates a new sandbox with a local copy of the repo,
so multiple sessions can work on independent branches in parallel.`,
```

with:

```go
rootCmd := &cobra.Command{
    Use:   "catvox [template] [workspace] [-- agent_args...]",
    Short: "Run Claude Code in sandboxed Docker containers",
    Long: `catvox creates isolated Docker sandbox environments for Claude Code
with per-template toolchains and network restrictions.

Each run creates a new sandbox with a local copy of the repo,
so multiple sessions can work on independent branches in parallel.`,
```

- [ ] **Step 2: Verify help output**

```bash
./catvox --help
```

Expected: usage line shows `catvox [template] [workspace] [-- agent_args...]` and description says `catvox creates isolated...`

- [ ] **Step 3: Commit**

```bash
git add cmd/catvox/main.go
git commit -m "refactor: update CLI help strings to catvox"
```

---

### Task 4: Update README.md

**Files:**
- Modify: `README.md` — full rewrite with new name section and all references

- [ ] **Step 1: Replace README.md with updated content**

Write the following to `README.md`:

```markdown
# catvox

**catvox** = **cat** + e**vo**lution + bo**x**

Inspired by Schrödinger's cat: you put a cat (your repo) in a box (a sandbox container) and let it evolve with Claude. 99% of the time the mutations are good — new features, refactors, fixes. But 1% of the time the cat dies (the agent goes rogue and the filesystem explodes). With catvox, the damage stays in the box. Throw it out, get a new box, get a new cat, start evolving again.

Without the box, the gas hits *you*.

---

Run [Claude Code](https://docs.anthropic.com/en/docs/claude-code) inside sandboxed [Docker containers](https://docs.docker.com/sandbox/) with per-template toolchains and network restrictions.

## Prerequisites

- [Go](https://go.dev/dl/) 1.21+
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) with sandbox support enabled

## Quick start

` ` `bash
# Run Claude Code in a JVM sandbox against the current directory
catvox jvm

# Run against a specific workspace
catvox jvm ~/projects/my-app

# Pass additional arguments to the agent
catvox jvm ~/projects/my-app -- -p "fix the tests"

# List all sandboxes
catvox ls

# Resume an existing sandbox (interactive picker)
catvox resume

# Resume with additional arguments
catvox resume -- -p "continue where you left off"

# Remove a specific sandbox
catvox rm myapp-jvm-sandbox-20260320-121500

# Remove all sandboxes for the current directory
catvox rm all
` ` `

## Installation

Build and symlink the binary onto your PATH:

` ` `bash
make build
ln -s /path/to/catvox/catvox /usr/local/bin/catvox
` ` `

Or build directly:

` ` `bash
go build -o catvox ./cmd/catvox
` ` `

## Templates

Each subdirectory under `templates/` with a `Dockerfile` is a template. Built-in templates:

| Template | What's included |
|----------|----------------|
| `jvm`    | Temurin JDK 21, Gradle/Maven repos, git-delta, fzf |

### Creating a template

1. Create a directory with a `Dockerfile` based on `docker/sandbox-templates:claude-code`.
2. Optionally add an `allowed-hosts.txt` to restrict network access (deny-by-default).

` ` `
templates/
  my-template/
    Dockerfile
    allowed-hosts.txt   # optional
` ` `

## Network policy

If a template contains `allowed-hosts.txt`, the sandbox uses a deny-by-default network policy — only the listed hosts are reachable. The policy is verified at creation time by confirming a blocked host is unreachable and an allowed host is reachable.

If no `allowed-hosts.txt` is present, the sandbox has unrestricted network access.

## How it works

1. Builds a Docker image from the template's `Dockerfile`.
2. Creates a named sandbox, mounting the repo and `~/.claude` config.
3. Symlinks the host `~/.claude` directory into the sandbox for auth and config.
4. Copies the repo to the container's local filesystem (`/home/agent/workspace/`) and creates a session branch. Docker's VirtioFS mounts on macOS have write-visibility latency that corrupts build caches — the local copy avoids this entirely.
5. Wraps the `claude` binary so Claude Code's project directory is the local copy — all tools (Edit, Read, Glob, Bash) operate on the same files.
6. Applies network restrictions if `allowed-hosts.txt` exists (with verification).
7. Runs Claude Code inside the sandbox with `--dangerously-skip-permissions`.

Each run creates a new sandbox with a fully local copy of the repo on its own branch, so multiple sessions can work independently in parallel.

## Development

### Running tests

` ` `bash
# Unit tests
make test

# Integration tests (requires Docker)
make test-integration

# Both
make test-all
` ` `

Prerequisites: Go 1.21+ and Docker (for integration tests).
```

Note: The triple backticks above are escaped for the plan. Use actual triple backticks in the file.

- [ ] **Step 2: Verify README renders correctly**

Visually inspect `README.md` — confirm the name explanation section is at the top, all `claudebox` references are now `catvox`, and code blocks are properly formatted.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update README with catvox branding and name explanation"
```

---

### Task 5: Update CLI examples in existing design docs

Per the spec: only update CLI usage examples and forward-looking references in old docs. Historical references stay as-is.

**Files:**
- Modify: `docs/superpowers/specs/2026-03-26-sandbox-naming-design.md:99`
- Modify: `docs/superpowers/specs/2026-03-25-go-migration-design.md:18-20,39-41,131,139,143`
- Modify: `docs/superpowers/specs/2026-03-24-integration-tests-design.md:121-122`
- Modify: `docs/superpowers/specs/2026-03-23-resume-sandbox-design.md:1,9,14,34,47,124`
- Modify: `docs/superpowers/specs/2026-03-23-decompose-claudebox-design.md:63`

- [ ] **Step 1: Update CLI examples in old specs**

In each file listed above, replace `claudebox jvm`, `claudebox ls`, `claudebox rm`, `claudebox resume`, and `claudebox` in CLI usage contexts with the `catvox` equivalent. Do NOT replace references that are clearly historical (e.g., titles like "Design: decompose claudebox" or narrative text describing the original project).

Specific replacements per file:

**`docs/superpowers/specs/2026-03-26-sandbox-naming-design.md`** line 99:
```
The same name appears in `catvox ls` and is used with `catvox rm` and `catvox resume`.
```

**`docs/superpowers/specs/2026-03-25-go-migration-design.md`** lines 18-20:
```
- `catvox resume` — resume an existing sandbox
- `catvox ls` — list all sandboxes
- `catvox rm <name|all>` — remove a sandbox by name, or `all` to remove all sandboxes for the current workspace
```

Lines 39-41:
```
│   │   ├── resume.go            # catvox resume
│   │   ├── ls.go                # catvox ls
│   │   └── rm.go                # catvox rm
```

Lines 131, 139, 143:
```
### `catvox resume`
...
### `catvox ls`
...
### `catvox rm <name|all>`
```

**`docs/superpowers/specs/2026-03-24-integration-tests-design.md`** lines 121-122:
```
- `catvox rm <name>` removes a specific sandbox (verified by `docker sandbox ls`)
- `catvox rm all` removes all sandboxes for a workspace (creates two, removes all, verifies none remain)
```

**`docs/superpowers/specs/2026-03-23-resume-sandbox-design.md`** — replace all CLI examples:
Line 1: `# Design: \`catvox resume\``
Line 9: `Add a \`catvox resume\` command...`
Line 14: `catvox resume [-- agent_args...]`
Line 34: `$ catvox resume`
Line 47: `$ catvox resume`
Line 124: `2. Run \`catvox resume\`, verify picker shows the sandbox.`

**`docs/superpowers/specs/2026-03-23-decompose-claudebox-design.md`** line 63:
```
- Validates `$# -lt 1` and prints inline usage (`Usage: catvox rm <sandbox-name|all>`) on failure
```

- [ ] **Step 2: Update CLI examples in old plans**

**`docs/superpowers/plans/2026-03-23-decompose-claudebox.md`** lines 117, 148, 205:
```
# catvox ls — list all sandboxes
...
# catvox rm — remove sandboxes
...
# catvox resume — resume an existing sandbox
```

**`docs/superpowers/plans/2026-03-23-resume-sandbox.md`** — replace CLI examples:
Line 1: `# \`catvox resume\` Implementation Plan`
Line 5: `**Goal:** Add a \`catvox resume\` command...`
Line 289: `git commit -m "feat: add catvox resume command with interactive picker"`
Line 301: `- [ ] **Step 2: Verify \`catvox resume\` with no sandboxes**`
Line 303: `Run: \`./catvox resume\`...`
Line 310: `3. Run \`./catvox resume\`...`
Line 317: `2. Run \`./catvox resume\`...`
Line 322: `1. Run \`./catvox resume\`...`

**`docs/superpowers/plans/2026-03-25-go-migration.md`** lines 30-31, 33:
```
| `internal/commands/ls.go` | `catvox ls` command |
| `internal/commands/rm.go` | `catvox rm` command |
...
| `internal/commands/resume.go` | `catvox resume` command |
```

Lines 1418, 1454, 1758 (code comments):
```go
// NewLsCmd returns the cobra command for catvox ls.
...
// NewRmCmd returns the cobra command for catvox rm.
...
// NewResumeCmd returns the cobra command for catvox resume.
```

- [ ] **Step 3: Commit**

```bash
git add docs/
git commit -m "docs: update CLI examples in old specs and plans to catvox"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run full unit test suite**

Run: `make test`
Expected: all tests pass

- [ ] **Step 2: Build the binary**

Run: `make build`
Expected: `catvox` binary produced

- [ ] **Step 3: Verify help output**

Run: `./catvox --help`
Expected: all output references `catvox`, no remaining `claudebox`

- [ ] **Step 4: Search for any remaining claudebox references in source code**

```bash
grep -r "claudebox" --include="*.go" --include="Makefile" --include=".gitignore" --include="README.md" .
```

Expected: no matches

- [ ] **Step 5: Search for remaining claudebox references in docs (should only be historical)**

```bash
grep -r "claudebox" docs/
```

Expected: only historical/narrative references remain (e.g., spec titles like "decompose-claudebox", file names). No CLI usage examples should reference `claudebox`.
