# Image Cache Eviction Design

## Problem

Docker Desktop's sandbox feature caches a ~773MB base image tar in
`~/.docker/sandboxes/image-cache/` every time `docker sandbox create` runs.
These tars are per-instance snapshots (unique hash suffix), not
content-addressed. Once the VM is running, it no longer references the tar —
the cache exists solely to speed up future creates.

Neither `docker sandbox rm` nor `docker system prune` removes these tars. Only
`docker sandbox reset` clears the cache, but it's all-or-nothing and kills
every sandbox. The result: unbounded cache growth. In practice this reached
211GB (234 tars) over ~one week of development.

## Design

On every `claudebox` invocation, before the actual command runs, prune
`~/.docker/sandboxes/image-cache/*.tar` files with a modification time older
than 1 hour.

### Why 1 hour

- The tar is only needed during `docker sandbox create`, which completes in
  under a minute.
- 1 hour provides a generous safety margin for slow creates or large workspaces
  being tar-piped in.
- At peak usage (~50 creates/hour), the worst-case accumulation before the next
  eviction pass is ~38GB — acceptable and self-correcting.

### Why on every command (not just `rm` or `create`)

- Decouples cleanup from any specific command's lifecycle.
- Handles orphans from crashes, force-kills, or manual `docker sandbox rm`
  outside claudebox.
- Single code path, no special-case logic in individual commands.

### Why not on `create` or `rm` specifically

- Parallel creates are common. Clearing the cache during `create` risks
  deleting a tar that a concurrent `docker sandbox create` is actively using.
- Tying cleanup to `rm` means the cache grows unbounded if the user never
  removes sandboxes.

## Implementation

A `PruneImageCache()` function called early in `main.go`, before cobra command
dispatch.

### Logic

1. Read `~/.docker/sandboxes/image-cache/` directory.
2. For each `.tar` file (skip `.tmp-*` files — these are active downloads):
   - Check `ModTime()`.
   - If older than 1 hour, `os.Remove()`.
3. Errors on individual deletes log to stderr but never block the command.
4. If the directory doesn't exist, return silently (Docker sandboxes not in
   use).

### Scope

- Single function, ~20 lines.
- Called once at startup in `main.go`.
- No bookkeeping, metadata files, or sandbox-to-tar mapping.
- No new CLI commands or flags.
- Does not touch `vm/`, `daemon.log`, or any other sandbox state.

### Testing

- Unit test: mock filesystem with tars of varying ages, assert only stale ones
  are removed and `.tmp-*` files are preserved.
