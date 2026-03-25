# claudebox

Run [Claude Code](https://docs.anthropic.com/en/docs/claude-code) inside sandboxed [Docker containers](https://docs.docker.com/sandbox/) with per-template toolchains and network restrictions.

## Prerequisites

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
2. Creates a named sandbox, mounting the repo and `~/.claude` config.
3. Symlinks the host `~/.claude` directory into the sandbox for auth and config.
4. Copies the repo to the container's local filesystem (`/home/agent/workspace/`) and creates a session branch. Docker's VirtioFS mounts on macOS have write-visibility latency that corrupts build caches — the local copy avoids this entirely.
5. Wraps the `claude` binary so Claude Code's project directory is the local copy — all tools (Edit, Read, Glob, Bash) operate on the same files.
6. Applies network restrictions if `allowed-hosts.txt` exists (with verification).
7. Runs Claude Code inside the sandbox with `--dangerously-skip-permissions`.

Each run creates a new sandbox with a fully local copy of the repo on its own branch, so multiple sessions can work independently in parallel.

## Development

### Running tests

```bash
# Unit tests
make test

# Integration tests (requires Docker)
make test-integration

# Both
make test-all
```

Prerequisites: Go 1.21+ and Docker (for integration tests).
