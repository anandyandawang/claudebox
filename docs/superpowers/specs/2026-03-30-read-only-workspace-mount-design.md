# Sandbox Host Isolation Design

**Date:** 2026-03-30 (revised 2026-03-31)
**Status:** Draft

## Summary

Eliminate all writable host mounts from the sandbox. Instead of mounting the workspace and `~/.claude` into the sandbox, copy only the needed files in via tar-pipe and mount an empty temp directory (which is immediately deleted on the host). This prevents the sandboxed agent from writing back to the host filesystem through any vector — direct writes, git operations, or inner Docker daemon re-mounts.

## Motivation

When running claudebox, the host workspace and `~/.claude` are bind-mounted read-write into the sandbox. Several attack vectors exist:

1. **Direct writes to host repo** — agent modifies the mounted host repo (e.g., rewrites sandbox creation code to mount the entire host filesystem)
2. **Inner Docker bypass of `:ro` mounts** — the inner Docker daemon has `CAP_SYS_ADMIN` and can re-mount virtiofs `:ro` mounts as `rw`, bypassing read-only protection. Writes propagate to the host.
3. **Host code execution via `~/.claude`** — agent writes a malicious plugin to `~/.claude/plugins/` or adds a hook to `~/.claude/settings.json` that executes on the host in the user's next Claude Code session

Making the mount read-only (`:ro`) only addresses vector 1. Vectors 2 and 3 require eliminating writable host mounts entirely.

## Design

### No host mounts

Replace all host mounts with tar-pipe file transfer and a dead temp directory mount:

| | Before | After |
|---|---|---|
| Primary workspace | Host repo (r/w) | Empty temp dir (deleted immediately after creation) |
| Secondary mount | `~/.claude` (r/w) | None |
| Workspace files | `cp -a` from virtiofs mount | `tar` piped through `docker sandbox exec -i` |
| Claude config | Symlinks to mounted `~/.claude` | Specific files copied via tar-pipe |

### Temp directory lifecycle

`docker sandbox create` requires a primary workspace argument. We satisfy this with a temp directory that is deleted immediately after sandbox creation:

1. Create temp dir on host (`os.MkdirTemp`)
2. Pass it as primary workspace to `docker sandbox create`
3. Delete the temp dir on the host immediately after creation
4. The virtiofs mount inside the sandbox becomes a dead end — the sandbox can `mkdir` in its overlay but writes never reach the host

Tested behavior after host-side deletion:
- `docker sandbox exec` still works (sandbox is alive)
- Virtiofs mount still appears in `findmnt` but points to nothing
- Direct writes fail: "No such file or directory"
- `mkdir` succeeds in sandbox overlay but directory does NOT re-appear on host
- Writes to the re-created directory still fail to reach the host

### File transfer via tar-pipe

Workspace and Claude config files are piped into the sandbox via `tar | docker sandbox exec -i`:

**Workspace:**
```
tar -C <workspace> -c . | docker sandbox exec -i <name> sh -c 'tar -C /home/agent/workspace -x'
```

**Claude config (specific files only):**
```
tar -C ~/.claude -c .claude.json settings.json plugins/ | docker sandbox exec -i <name> sh -c 'tar -C /home/agent/.claude -x'
```

Only three items are copied from `~/.claude`:
- `.claude.json` — Claude configuration
- `settings.json` — Claude Code settings
- `plugins/` — installed plugins

Performance: tar-pipe is equivalent to `cp -a` from virtiofs (~1.5s for a 10MB, 1200-file repo).

The `-i` flag on `docker sandbox exec` is required for stdin piping.

### Resume behavior

On resume, refresh Claude config files from the host:

1. Re-copy `settings.json`, `plugins/` from host (picks up new plugins or setting changes)
2. Continue refreshing credentials (existing behavior)

This means plugins are read-only from the sandbox's perspective — the host is the source of truth. Installing a plugin inside the sandbox only affects that sandbox. Installing on the host and resuming syncs the new plugin in.

### What stays the same

- `git clean -fdx && git checkout -b <session>` in the workspace copy
- Network policy, environment setup, binary wrapping
- `docker sandbox run` for interactive sessions
- All CLI commands and user-facing behavior

### What changes

- No more symlinks for Claude config (files are copied, not symlinked)
- Plugin/settings changes inside the sandbox don't persist to the host
- New plugins installed on the host are picked up on resume

## Code Changes

### `internal/docker/docker.go`

Add `SandboxExecWithStdin` method for tar-pipe support:

```go
func (c *Client) SandboxExecWithStdin(r io.Reader, name string, args ...string) error {
    cmdArgs := append([]string{"sandbox", "exec", "-i", name}, args...)
    cmd := c.newCmd("docker", cmdArgs...)
    cmd.Stdin = r
    return cmd.Run()
}
```

Simplify `SandboxCreateOpts` — only needs an image, command, and single workspace path:

```go
type SandboxCreateOpts struct {
    Image     string // Docker image tag
    Command   string // Base command (e.g. "claude")
    Workspace string // Primary workspace path (temp dir, deleted after creation)
}
```

### `internal/sandbox/sandbox.go`

Update `Create` to:
1. Create a temp dir
2. Pass it as the workspace to `docker sandbox create`
3. Delete the temp dir
4. Tar-pipe the workspace files into `/home/agent/workspace/`
5. Tar-pipe Claude config files into `/home/agent/.claude/`
6. Run `git clean -fdx && git checkout -b <session>`

Update `Resume` (or add a `RefreshConfig` method) to:
1. Re-copy `settings.json` and `plugins/` from host via tar-pipe

### Unit tests

Update `internal/docker/docker_test.go` and `internal/sandbox/sandbox_test.go` to cover:
- `SandboxExecWithStdin` pipes data correctly
- `Create` uses temp dir, tar-pipes workspace and config
- Resume refreshes config

## Integration Tests

### Workspace isolation

Verify the host workspace path does not exist inside the sandbox:

1. **Workspace path not mounted** — the host workspace path does not appear in `findmnt` output inside the sandbox
2. **Workspace copy is writable** — `touch /home/agent/workspace/test-file` succeeds
3. **Workspace copy has repo files** — test files from the workspace are present

### Dead mount escape attempts

Verify the deleted temp dir mount cannot be exploited:

4. **Write to dead mount fails** — `touch <dead-mount>/file` fails
5. **Re-create dir in sandbox doesn't propagate to host** — `mkdir -p <dead-mount>` succeeds inside sandbox but directory does NOT appear on host
6. **Write to re-created dir doesn't propagate to host** — `touch <dead-mount>/file` after `mkdir` does not create file on host

### Host Docker daemon isolation

7. **Host socket not accessible** — `stat /var/run/docker.sock` fails, or the socket is the sandbox's own daemon
8. **Can't reach host Docker daemon over TCP** — `curl host.docker.internal:2375` fails

### Docker/VM boundary isolation

9. **Host paths not reachable via inner Docker** — `docker run -v /Users:/test alpine ls /test` shows VM-local content, not host home directories
10. **Mount root is VM-scoped** — `docker run -v /:/mnt alpine cat /mnt/etc/hostname` returns sandbox hostname, not host's

### Inner Docker escape attempt

The inner Docker daemon has `CAP_SYS_ADMIN` and can re-mount virtiofs mounts as r/w. Verify this cannot be exploited to write to the host:

11. **Inner Docker can't write to dead mount** — `docker run -v <dead-mount>:/repo alpine touch /repo/file` does not create file on host
12. **Inner Docker can't write to arbitrary host paths** — `docker run -v /tmp:/t alpine touch /t/escape-marker` does not create file on host `/tmp`

### Sandbox escape test

Comprehensive escape attempt — write a script inside the sandbox and attempt to execute it on the host:

13. **Script written to dead mount doesn't execute on host** — write a shell script that creates a marker file, attempt to place it somewhere that auto-executes (cron, launchd, shell profile paths). Verify marker file never appears on host.

## Security Analysis

### Attack vectors closed

| Vector | Before | After |
|---|---|---|
| Direct write to host repo | Possible (r/w mount) | Impossible (no mount) |
| Inner Docker bypass of `:ro` | Possible (`CAP_SYS_ADMIN` re-mount) | Impossible (no mount to re-mount) |
| Malicious plugin in `~/.claude/plugins/` | Possible (r/w mount) | Impossible (no mount) |
| Malicious hook in `~/.claude/settings.json` | Possible (r/w mount) | Impossible (no mount) |
| Write to dead temp dir mount | N/A | Impossible (virtiofs bridge severed) |
| Re-create dead mount dir | N/A | Dir exists in sandbox overlay only, not on host |

### Remaining acceptable risk

- If the developer ships claudebox code that mounts dangerous host paths, that's a software bug, not a sandbox escape
- The agent can modify its local copy of settings/plugins but these don't persist to the host

### Defense in depth

- No writable host mounts (primary defense)
- Sandbox runs in a microVM with its own Docker daemon
- Host Docker socket not accessible from inside sandbox
- Inner Docker daemon can only mount VM-local paths
- Network policies restrict outbound access
- Dead temp dir mount is permanently severed after host-side deletion

## Scope

This changes how files enter the sandbox (tar-pipe instead of mount + copy) and how Claude config is managed (copy instead of symlink). CLI commands and user-facing behavior are unchanged. Plugin/settings changes inside the sandbox are sandbox-local; host changes sync on resume.
