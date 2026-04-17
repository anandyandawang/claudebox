# Import Host Git Identity Into Sandbox

**Date:** 2026-04-17
**Status:** Approved

## Summary

Import the host's global git identity (`user.name` and `user.email`) into the sandbox at create time so commits made inside the sandbox are attributed to the user. Values are read from the host's `git config --global` and written to the sandbox's `~/.gitconfig` via `git config --global` inside the container. Missing values on the host are silently skipped.

## Motivation

The sandbox tar-pipes the repo in, so any per-repo identity in the workspace's `.git/config` comes along. But the host's **global** git identity does not — meaning any commit made in the sandbox against a repo without a local `user.name`/`user.email` has no identity and either fails or picks up the sandbox user's default. Users have to manually re-run `git config --global` inside the sandbox on every create.

`environment.Setup()` already imports `GITHUB_USERNAME` on create via the same "read host value, push into sandbox, silently skip if missing" pattern. Extending it to git identity closes the gap consistently.

## Design

### Placement

Extend `internal/environment/environment.go`. Add a helper that reads the host values and writes them into the sandbox, called from `Setup()` alongside the existing `GITHUB_USERNAME` and JVM-proxy blocks. `Setup()` runs only on create (per the 2026-04-02 ADR), which is the correct cadence — git identity is host-stable and does not need refreshing on resume.

### Flow

1. On the host, read each value independently using a replaceable function (`readGitIdentityFn`, following the `readKeychainFn` pattern in `internal/credentials/keychain.go` for testability):
   - `git config --global user.name` → trimmed; empty on error or unset.
   - `git config --global user.email` → trimmed; empty on error or unset.
2. For each non-empty value, run inside the sandbox:
   - `d.SandboxExec(sandboxName, "git", "config", "--global", "user.name", "<value>")`
   - `d.SandboxExec(sandboxName, "git", "config", "--global", "user.email", "<value>")`
3. Args pass as separate slice elements (not `sh -c`), so shell escaping is not needed.

Result: values land in `/home/agent/.gitconfig` inside the sandbox. Any tool that queries `git config user.name`/`user.email` — including `git commit`, `gh`, editors, hooks — picks them up naturally. Per-repo overrides from the tar-piped `.git/config` still win, matching host semantics.

### Error handling

- `git` not installed on host, or the config key unset → `exec.Command` returns error or empty output → silently skip that key. Matches the existing `GITHUB_USERNAME` "skip if unset" behavior.
- Only one of name/email is set → set just the one that exists. Partial identity is valid git behavior.
- Sandbox `git config` call fails → return the error from `Setup()`, same as the other steps in that function.
- No warning is printed for missing host values. The rest of the patterns in `environment.Setup()` are silent; consistency wins over user education here.

### Out of scope

- Other gitconfig keys (`user.signingkey`, `commit.gpgsign`, `pull.rebase`, etc.). Signing in particular requires a gpg key that is not imported into the sandbox; importing only the config without the key would break `git commit -S`.
- Refreshing identity on resume. Global gitconfig persists across stop/start inside the sandbox; there is no need to re-import.
- Reading effective host config (`git config user.name` without `--global`, which merges system + global + local). Per-repo `.git/config` is already tar-piped, so global + local is the correct and sufficient source.

## Testing

### Unit tests

In `internal/environment/environment_test.go`, using the existing `mockDocker` and a replaceable `readGitIdentityFn` seam:

- Both name and email set → two `SandboxExec` calls with the expected `git config --global user.<key> <value>` args.
- Only name set → one call, for name.
- Only email set → one call, for email.
- Neither set → no git-config calls issued.

### Integration tests

In `tests/integration/filesystem_test.go`, add three subtests under `TestFilesystemLayout`. All three close a pre-existing gap: `environment.Setup()` runs in every integration test via `createTestSandbox`, but nothing today verifies its side effects inside the container.

1. **`host git identity imported into sandbox`**
   - Read `git config --global user.name` and `user.email` on the host with `exec.Command`.
   - If both are empty, `t.Skip("host has no global git identity")` — CI runners may not have one configured.
   - For each non-empty host value: run `git config --global user.<key>` inside the sandbox via `testDocker.SandboxExec`; assert the trimmed output equals the host value.

2. **`GITHUB_USERNAME exported in sandbox-persistent.sh`**
   - If `GITHUB_USERNAME` is not set on the host, `t.Skip("GITHUB_USERNAME not set on host")`.
   - Inside the sandbox: source `/etc/sandbox-persistent.sh` in a subshell and print the value, e.g. `sh -c '. /etc/sandbox-persistent.sh && printf %s "$GITHUB_USERNAME"'`.
   - Assert the trimmed output equals the host value.

3. **`JAVA_TOOL_OPTIONS written when HTTPS_PROXY is set on host`**
   - If `HTTPS_PROXY` is not set on the host, `t.Skip("HTTPS_PROXY not set on host")`.
   - Parse the expected proxy host and port from the host's `HTTPS_PROXY` (matching the `sed` transforms in `environment.go`).
   - Inside the sandbox: source `/etc/sandbox-persistent.sh` in a subshell and print `$JAVA_TOOL_OPTIONS`.
   - Assert the output contains `-Dhttps.proxyHost=<expected>` and `-Dhttps.proxyPort=<expected>` (and the corresponding `http.proxyHost`/`http.proxyPort`).

Together these validate the observable contracts of `environment.Setup()` — the pre-existing env-var contract (`GITHUB_USERNAME`), the new git-identity contract, and the JVM proxy contract — by actually reading from inside the container.

**Not covered at integration level:** the keytool CA import branch. On a fresh `jvm` template image, `/usr/local/share/ca-certificates/` is empty so the branch never fires, and adding integration coverage would require planting a fake CA cert into the running sandbox before re-invoking setup — more scaffolding than the coverage earns for a branch that is already guarded by an `|| true`. Unit-level coverage in `environment_test.go` remains the mechanism for that path.

## Side effects

- `TestFilesystemLayout` gains real coverage of `environment.Setup()`. Future additions to `Setup()` will sit next to a test file that already asserts on container-side state, making regressions for the existing `GITHUB_USERNAME` path catchable alongside any new additions.
