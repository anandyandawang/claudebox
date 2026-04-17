# Sync to origin default branch on sandbox create

## Problem

When `claudebox <template>` creates a sandbox, the session branch is based on whatever the host happens to have checked out at that moment — a possibly-stale branch, possibly with uncommitted work. Users routinely want to start a new session from the latest state of the repository's default branch instead.

## Decision

On **create**, after the workspace has been tar-piped into the sandbox:

1. Discover the default branch by asking the remote.
2. Fetch from origin.
3. Reset the working tree to `origin/<default>`, silently discarding any local modifications and untracked files carried over from the host.
4. Create the session branch from that state.

On **resume**, nothing changes. The existing session branch has in-progress work; auto-fetching or auto-resetting would be either surprising or destructive.

## New sandbox create flow

The tar-pipe and config-copy steps are unchanged. The block that currently runs in `Manager.Create()` at `internal/sandbox/sandbox.go:87-92`:

```
git clean -fdx -q
git checkout -b <sessionID>
```

is replaced with:

```
git ls-remote --symref origin HEAD          # discover default branch
git clean -fdx -q                           # drop untracked (tar-pipe cruft)
git fetch origin
git checkout -f -B <default> origin/<default>   # -f silently drops tracked mods
git checkout -b <sessionID>
```

All commands run inside the sandbox via `m.docker.SandboxExec`. Git auth inside the sandbox is already handled externally — no credential injection is part of this change.

### Why this order

- `ls-remote` runs first so we fail fast if the repo has no `origin` or the remote isn't reachable, *before* we modify the sandbox's working tree.
- `git clean` removes untracked (and ignored) files from the tar-pipe. Needed because the subsequent checkout only touches tracked files — untracked cruft would otherwise survive into the session branch.
- `git checkout -f -B` force-resets the local default branch to `origin/<default>` and switches HEAD to it. The `-f` drops tracked-file modifications silently (matches the "silent reset" decision below).
- Final `git checkout -b <sessionID>` creates the session branch from the now-clean `<default>` state.

## Default branch discovery

Use `git ls-remote --symref origin HEAD`, parsed for a line of the form:

```
ref: refs/heads/<name>\tHEAD
```

Extract `<name>`. Always consult the remote — don't rely on the local `origin/HEAD` symbolic-ref, which can be absent or stale for shallow clones or manually-added remotes.

## Edge cases

| Case | Behavior |
|------|----------|
| Workspace has no `origin` remote | Abort create with wrapped error from `ls-remote`. |
| `ls-remote` fails (network, auth) | Abort create with wrapped error. |
| `fetch` fails | Abort create. |
| `checkout -B` fails | Abort create. |
| Host has uncommitted tracked changes | Silently discarded (per `-f`). |
| Host has untracked files | Silently discarded (per `git clean -fdx`). |
| Host is on a non-default branch | Irrelevant — session is based on `origin/<default>` regardless. |
| Workspace is not a git repo | Already fails today at `git clean`; no new handling needed. |

"Abort" means: the create command returns an error; the sandbox that was created up to that point is left in place for inspection. (This matches existing behavior for other mid-create failures like network policy verification.)

## Code organization

Add one method on `sandbox.Manager`:

```go
// resetToDefaultBranch discovers origin's default branch, fetches, and force-resets
// the sandbox's working tree to origin/<default>. Leaves HEAD on <default>.
func (m *Manager) resetToDefaultBranch(sandboxName string) error
```

Add one unexported parsing helper, easy to unit-test without Docker:

```go
// parseDefaultBranchFromSymref extracts the branch name from
// `git ls-remote --symref origin HEAD` output.
func parseDefaultBranchFromSymref(output string) (string, error)
```

`Manager.Create()` calls `resetToDefaultBranch` between the tar-pipe steps and the session-branch checkout. The existing `git clean` call is absorbed into `resetToDefaultBranch`.

## Testing

### Unit tests (`internal/sandbox/sandbox_test.go`)

Mock `docker.SandboxExec` with canned outputs per command.

- `parseDefaultBranchFromSymref`: normal output, trailing whitespace/newline, malformed output (error path).
- `resetToDefaultBranch` happy path: asserts exact command sequence.
- Error propagation: each of `ls-remote`, `fetch`, `checkout -B` failing surfaces the error with the right wrapping prefix.

### Integration tests

**Fixture location.** A bare repo serves as origin. It must be reachable from inside the sandbox, so it rides in via tar-pipe; the only tar-piped location is the workspace. Putting the bare at the workspace root would get wiped by `git clean -fdx`, so it lives inside `.git/` instead (git never cleans its own git dir). Final path:

```
<workspace>/.git/integration-test-origin.git/
```

Origin URL (set on host, consumed inside the sandbox):
```
file:///home/agent/workspace/.git/integration-test-origin.git
```

**New helper** in `tests/integration/helpers_test.go`:

```go
// createTestWorkspaceWithBareOrigin builds a git workspace wired up to a local
// bare repo as "origin". The bare lives inside .git/ so it's carried into the
// sandbox by tar-pipe and survives git clean. Reads via file:// inside the
// sandbox; no network.
func createTestWorkspaceWithBareOrigin(t *testing.T, dirname string) string
```

**Tests** (in `tests/integration/filesystem_test.go`):

1. **Happy path**: create a sandbox from a workspace with a bare origin. Assert the session branch's tip matches `origin/main`'s tip.
2. **No origin**: create a workspace with `git init` but no `origin` remote; `claudebox create` aborts with a clear error mentioning `origin`.

**Migration**: the existing `createTestWorkspace` helper produces workspaces with no remote. With abort-on-no-origin in place, any integration test that calls create on such a workspace will start failing. Audit the callers of `createTestWorkspace` and migrate them to `createTestWorkspaceWithBareOrigin`, unless the test is specifically exercising the no-origin abort path.

The "origin has commits the host doesn't" scenario is covered by unit tests (mocking `docker.SandboxExec`) rather than integration, because simulating divergence requires a second ephemeral clone-commit-push dance that adds confusion without buying much coverage beyond what mocks already give.

## Non-goals

- No changes to resume.
- No git-credential injection (assumed handled externally).
- No changes to tar-pipe, network policy, or any other step of create.
- No CLI flag to override the default branch — the remote is the source of truth.
- No CLI flag to opt out of the fetch-and-reset — it's always-on by design.
