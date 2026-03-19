# claudebox

Run [Claude Code](https://docs.anthropic.com/en/docs/claude-code) inside sandboxed [Docker containers](https://docs.docker.com/sandbox/) with per-template toolchains and network restrictions.

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) with sandbox support enabled

## Quick start

```bash
# Run Claude Code in a Python sandbox against the current directory
claudebox python

# Run in a JVM sandbox against a specific workspace
claudebox jvm ~/projects/my-app

# Pass additional arguments to the agent
claudebox python ~/projects/my-app -- -p "fix the tests"
```

## Installation

Symlink the script onto your PATH:

```bash
ln -s /path/to/claudebox/claudebox /usr/local/bin/claudebox
```

## Templates

Each subdirectory with a `Dockerfile` is a template. Built-in templates:

| Template | What's included |
|----------|----------------|
| `python` | Python 3, pip, pytest, black, pylint |
| `jvm`    | Temurin JDK 21, Gradle/Maven repos, git-delta, fzf |

### Creating a template

1. Create a directory with a `Dockerfile` based on `docker/sandbox-templates:claude-code`.
2. Optionally add an `allowed-hosts.txt` to restrict network access (deny-by-default).

```
my-template/
  Dockerfile
  allowed-hosts.txt   # optional
```

## Network policy

If a template contains `allowed-hosts.txt`, the sandbox uses a deny-by-default network policy — only the listed hosts are reachable. The policy is verified at creation time by confirming a blocked host is unreachable and an allowed host is reachable.

If no `allowed-hosts.txt` is present, the sandbox has unrestricted network access.

## How it works

1. Builds a Docker image from the template's `Dockerfile`.
2. Creates a named sandbox, mounting your workspace and `~/.claude` config.
3. Symlinks host Claude credentials, settings, and plugins into the sandbox.
4. Applies network restrictions if `allowed-hosts.txt` exists (with verification).
5. Runs Claude Code inside the sandbox with `--dangerously-skip-permissions`.

Subsequent runs reuse the existing sandbox if one with the same name is found.
