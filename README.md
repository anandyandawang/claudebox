# claudebox

Run [Claude Code](https://docs.anthropic.com/en/docs/claude-code) inside sandboxed [Docker containers](https://docs.docker.com/sandbox/) with per-template toolchains and network restrictions.

## Prerequisites

- [Go](https://go.dev/dl/) 1.21+
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) with sandbox support enabled

## Quick start

```bash
# Run Claude Code in a JVM sandbox against the current directory
claudebox jvm

# Run against a specific workspace
claudebox jvm ~/projects/my-app

# Pass additional arguments to the agent
claudebox jvm ~/projects/my-app -- -p "fix the tests"

# List all sandboxes
claudebox ls

# Resume an existing sandbox (interactive picker)
claudebox resume

# Resume with additional arguments
claudebox resume -- -p "continue where you left off"

# Remove a specific sandbox
claudebox rm myapp-jvm-sandbox-20260320-121500

# Remove all sandboxes for the current directory
claudebox rm all
```

## Installation

Build and symlink the binary onto your PATH:

```bash
make build
ln -s /path/to/claudebox/claudebox /usr/local/bin/claudebox
```

Or build directly:

```bash
go build -o claudebox ./cmd/claudebox
```

## Templates

Each subdirectory under `templates/` with a `Dockerfile` is a template. Built-in templates:

| Template | What's included |
|----------|----------------|
| `jvm`    | Temurin JDK 21, Gradle/Maven repos, git-delta, fzf |

### Creating a template

1. Create a directory with a `Dockerfile` based on `docker/sandbox-templates:claude-code`.
2. Optionally add an `allowed-hosts.txt` to restrict network access (deny-by-default).

```
templates/
  my-template/
    Dockerfile
    allowed-hosts.txt   # optional
```

## Network policy

If a template contains `allowed-hosts.txt`, the sandbox uses a deny-by-default network policy — only the listed hosts are reachable. The policy is verified at creation time by confirming a blocked host is unreachable and an allowed host is reachable.

If no `allowed-hosts.txt` is present, the sandbox has unrestricted network access.

## How it works

1. Builds a Docker image from the template's `Dockerfile`.
2. Creates a named sandbox with an empty temporary directory as the workspace mount — the real workspace files are streamed in separately, so the mount never contains sensitive data.
3. Tar-pipes the repo into `/home/agent/workspace/` and Claude config files (`.claude.json`, `settings.json`, `plugins/`) into `/home/agent/.claude/` via `docker sandbox exec -i`.
4. Creates a session branch in the workspace copy.
5. Wraps the `claude` binary so Claude Code's project directory is the local copy — all tools (Edit, Read, Glob, Bash) operate on the same files.
6. Applies network restrictions if `allowed-hosts.txt` exists (with verification).
7. Runs Claude Code inside the sandbox with `--dangerously-skip-permissions`.

Each run creates a new sandbox with a fully local copy of the repo on its own branch, so multiple sessions can work independently in parallel. On resume, settings and plugins are re-synced from the host.

### Host isolation

The sandbox has no writable mounts back to the host filesystem:

- **Empty temp dir mount** — the required VirtioFS workspace mount points at an empty host temp directory, not the real workspace. Writes inside the sandbox go to this empty dir, never to the real workspace.
- **Tar-pipe file transfer** — workspace files and Claude config are streamed into the sandbox via `tar | docker sandbox exec -i`, not mounted. Changes inside the sandbox are sandbox-local.
- **No host Docker access** — the sandbox runs inside a Docker Desktop VM with its own Docker daemon. Inner containers cannot mount host paths or communicate with the host Docker daemon.

## Development

### Running tests

```bash
# Unit tests
make test

# Integration tests (requires Docker Desktop with sandbox support)
make test-integration

# Both
make test-all
```

Prerequisites: Go 1.21+ and Docker Desktop with sandbox support (for integration tests).

### Test structure

| Suite | Location | What it covers |
|-------|----------|---------------|
| Unit tests | `internal/*/` | Docker client, sandbox lifecycle, create/resume commands, credentials, environment setup |
| Integration: filesystem | `tests/integration/filesystem_test.go` | Workspace layout, git branch, config symlinks, claude wrapper |
| Integration: network | `tests/integration/network_test.go` | Deny-by-default firewall, allowed hosts, no-policy fallback |
| Integration: security | `tests/integration/security_test.go` | Host isolation, dead mount escapes, Docker daemon isolation, escape attempts |
