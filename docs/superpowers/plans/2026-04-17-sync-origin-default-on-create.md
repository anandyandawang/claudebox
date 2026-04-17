# Sync to origin default branch on sandbox create — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On `claudebox <template>`, reset the sandbox's working tree to the latest `origin/<default>` before creating the session branch, so sessions always start from a fresh default-branch base instead of inheriting the host's possibly-stale checkout.

**Architecture:** Inside `Manager.Create()` (after tar-pipe, before session branch), call a new `resetToDefaultBranch` helper that asks origin for the default branch name, fetches, and force-resets the working tree to `origin/<default>`. Resume is unchanged.

**Tech Stack:** Go, `git` CLI, `testing` package, Docker Desktop sandbox (for integration tests).

**Spec:** `docs/superpowers/specs/2026-04-17-sync-origin-default-on-create-design.md`

---

## File Structure

- `internal/sandbox/sandbox.go` — add `parseDefaultBranchFromSymref` helper and `resetToDefaultBranch` method; call it from `Create()`.
- `internal/sandbox/sandbox_test.go` — extend `mockDocker` with per-command error support; add parser tests and `resetToDefaultBranch` tests; update `TestCreate` to seed ls-remote output.
- `tests/integration/helpers_test.go` — add `createTestWorkspaceWithBareOrigin` helper.
- `tests/integration/create_test.go`, `filesystem_test.go`, `network_test.go`, `security_test.go` — migrate callers of `createTestWorkspace` that need an origin to the new helper.
- `tests/integration/filesystem_test.go` — add happy-path integration test (session branch == origin/main tip) and no-origin abort test.

---

### Task 1: Add `parseDefaultBranchFromSymref` helper (TDD)

**Files:**
- Modify: `internal/sandbox/sandbox.go` (add unexported function)
- Modify: `internal/sandbox/sandbox_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Append to `internal/sandbox/sandbox_test.go`:

```go
func TestParseDefaultBranchFromSymref(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"main", "ref: refs/heads/main\tHEAD\nabc123\tHEAD\n", "main", false},
		{"master", "ref: refs/heads/master\tHEAD\nabc123\tHEAD\n", "master", false},
		{"trailing newline only", "ref: refs/heads/develop\tHEAD\n", "develop", false},
		{"no trailing newline", "ref: refs/heads/develop\tHEAD", "develop", false},
		{"branch with slash", "ref: refs/heads/feature/foo\tHEAD\n", "feature/foo", false},
		{"empty", "", "", true},
		{"malformed", "not-a-ref-line\nblah\n", "", true},
		{"ref prefix but missing branch", "ref: refs/heads/\tHEAD\n", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDefaultBranchFromSymref(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestParseDefaultBranchFromSymref -v`
Expected: FAIL with "undefined: parseDefaultBranchFromSymref"

- [ ] **Step 3: Implement the helper**

Append to `internal/sandbox/sandbox.go` (at end of file):

```go
// parseDefaultBranchFromSymref extracts the branch name from the output of
// `git ls-remote --symref origin HEAD`. The first line is expected to be of
// the form "ref: refs/heads/<branch>\tHEAD".
func parseDefaultBranchFromSymref(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		const prefix = "ref: refs/heads/"
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimPrefix(line, prefix)
		fields := strings.Fields(rest)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		return fields[0], nil
	}
	return "", fmt.Errorf("could not parse default branch from ls-remote output: %q", output)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestParseDefaultBranchFromSymref -v`
Expected: PASS (all subtests)

- [ ] **Step 5: Commit**

```bash
cd /home/agent/workspace
git add internal/sandbox/sandbox.go internal/sandbox/sandbox_test.go
git commit -m "feat: add parseDefaultBranchFromSymref helper

Extracts the default branch name from git ls-remote --symref output.
Used by the upcoming resetToDefaultBranch step in sandbox create."
```

---

### Task 2: Extend `mockDocker` with per-command error support

**Files:**
- Modify: `internal/sandbox/sandbox_test.go:19-56` (the `mockDocker` struct and its `SandboxExec` method)

Rationale: existing `failOn: "SandboxExec"` fails every exec call indiscriminately. `resetToDefaultBranch` tests need to simulate "ls-remote succeeds but fetch fails" etc.

- [ ] **Step 1: Add an `execErrs` map to the mock**

Update the `mockDocker` struct declaration at `internal/sandbox/sandbox_test.go:19-24`:

```go
type mockDocker struct {
	calls    []call
	execOut  map[string]string
	execErrs map[string]error // if joined args contain key, return this error
	lsOutput []docker.SandboxInfo
	failOn   string
}
```

- [ ] **Step 2: Consult `execErrs` inside `SandboxExec`**

Replace the `SandboxExec` method body at `internal/sandbox/sandbox_test.go:47-56`:

```go
func (m *mockDocker) SandboxExec(name string, args ...string) (string, error) {
	m.record("SandboxExec", append([]string{name}, args...)...)
	if m.failOn == "SandboxExec" { return "", fmt.Errorf("exec failed") }
	joined := strings.Join(args, " ")
	for substr, err := range m.execErrs {
		if strings.Contains(joined, substr) {
			return "", err
		}
	}
	for prefix, out := range m.execOut {
		if strings.Contains(joined, prefix) {
			return out, nil
		}
	}
	return "", nil
}
```

- [ ] **Step 3: Run unit tests to verify no regressions**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -v`
Expected: all existing tests still PASS. (The new map is nil-by-default and the ranged loop over a nil map is a no-op.)

- [ ] **Step 4: Commit**

```bash
cd /home/agent/workspace
git add internal/sandbox/sandbox_test.go
git commit -m "test: add execErrs to mockDocker for per-command failure injection"
```

---

### Task 3: Add `resetToDefaultBranch` method (TDD — happy path)

**Files:**
- Modify: `internal/sandbox/sandbox.go` (add method)
- Modify: `internal/sandbox/sandbox_test.go` (add test)

- [ ] **Step 1: Write the happy-path test**

Append to `internal/sandbox/sandbox_test.go`:

```go
func TestResetToDefaultBranch(t *testing.T) {
	m := &mockDocker{
		execOut: map[string]string{
			"ls-remote --symref": "ref: refs/heads/main\tHEAD\nabc123\tHEAD\n",
		},
	}
	mgr := NewManager(m, "/templates")

	if err := mgr.resetToDefaultBranch("test-sandbox"); err != nil {
		t.Fatal(err)
	}

	// Verify the command sequence issued via SandboxExec.
	var execArgs [][]string
	for _, c := range m.calls {
		if c.method == "SandboxExec" {
			execArgs = append(execArgs, c.args)
		}
	}
	if len(execArgs) != 4 {
		t.Fatalf("expected 4 SandboxExec calls, got %d: %v", len(execArgs), execArgs)
	}

	checks := []struct {
		name     string
		contains []string
	}{
		{"ls-remote first", []string{"ls-remote", "--symref", "origin", "HEAD"}},
		{"clean second", []string{"clean", "-fdx", "-q"}},
		{"fetch third", []string{"fetch", "origin"}},
		{"checkout -f -B fourth", []string{"checkout", "-f", "-B", "main", "origin/main"}},
	}
	for i, chk := range checks {
		joined := strings.Join(execArgs[i], " ")
		for _, want := range chk.contains {
			if !strings.Contains(joined, want) {
				t.Errorf("%s: args[%d]=%q missing %q", chk.name, i, joined, want)
			}
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestResetToDefaultBranch -v`
Expected: FAIL with "mgr.resetToDefaultBranch undefined"

- [ ] **Step 3: Implement the method**

Append to `internal/sandbox/sandbox.go` (before the `parseDefaultBranchFromSymref` helper added in Task 1, so methods group with methods):

```go
// resetToDefaultBranch discovers origin's default branch, fetches, and force-resets
// the sandbox's working tree to origin/<default>. HEAD ends up on <default>.
// Silently discards any local modifications and untracked files carried in via
// tar-pipe.
func (m *Manager) resetToDefaultBranch(sandboxName string) error {
	out, err := m.docker.SandboxExec(sandboxName, "git", "-C", SandboxWorkspace,
		"ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return fmt.Errorf("determining default branch: %w", err)
	}
	branch, err := parseDefaultBranchFromSymref(out)
	if err != nil {
		return fmt.Errorf("determining default branch: %w", err)
	}

	if _, err := m.docker.SandboxExec(sandboxName, "git", "-C", SandboxWorkspace,
		"clean", "-fdx", "-q"); err != nil {
		return fmt.Errorf("cleaning workspace: %w", err)
	}

	if _, err := m.docker.SandboxExec(sandboxName, "git", "-C", SandboxWorkspace,
		"fetch", "origin"); err != nil {
		return fmt.Errorf("fetching origin: %w", err)
	}

	if _, err := m.docker.SandboxExec(sandboxName, "git", "-C", SandboxWorkspace,
		"checkout", "-f", "-B", branch, "origin/"+branch); err != nil {
		return fmt.Errorf("resetting to origin/%s: %w", branch, err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestResetToDefaultBranch -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/agent/workspace
git add internal/sandbox/sandbox.go internal/sandbox/sandbox_test.go
git commit -m "feat: add Manager.resetToDefaultBranch

Runs ls-remote → clean → fetch → force-checkout inside the sandbox.
Not yet wired into Create()."
```

---

### Task 4: Add error-path tests for `resetToDefaultBranch`

**Files:**
- Modify: `internal/sandbox/sandbox_test.go` (add tests)

- [ ] **Step 1: Write error-path tests**

Append to `internal/sandbox/sandbox_test.go`:

```go
func TestResetToDefaultBranchLsRemoteFails(t *testing.T) {
	m := &mockDocker{
		execErrs: map[string]error{
			"ls-remote --symref": fmt.Errorf("no such remote: origin"),
		},
	}
	mgr := NewManager(m, "/templates")

	err := mgr.resetToDefaultBranch("test-sandbox")
	if err == nil {
		t.Fatal("expected error when ls-remote fails")
	}
	if !strings.Contains(err.Error(), "determining default branch") {
		t.Errorf("error should mention default branch discovery, got: %v", err)
	}
}

func TestResetToDefaultBranchMalformedLsRemote(t *testing.T) {
	m := &mockDocker{
		execOut: map[string]string{"ls-remote --symref": "garbage\n"},
	}
	mgr := NewManager(m, "/templates")

	err := mgr.resetToDefaultBranch("test-sandbox")
	if err == nil {
		t.Fatal("expected error when ls-remote output is malformed")
	}
	if !strings.Contains(err.Error(), "determining default branch") {
		t.Errorf("error should mention default branch discovery, got: %v", err)
	}
}

func TestResetToDefaultBranchFetchFails(t *testing.T) {
	m := &mockDocker{
		execOut: map[string]string{
			"ls-remote --symref": "ref: refs/heads/main\tHEAD\n",
		},
		execErrs: map[string]error{
			"fetch origin": fmt.Errorf("network error"),
		},
	}
	mgr := NewManager(m, "/templates")

	err := mgr.resetToDefaultBranch("test-sandbox")
	if err == nil {
		t.Fatal("expected error when fetch fails")
	}
	if !strings.Contains(err.Error(), "fetching origin") {
		t.Errorf("error should mention fetching origin, got: %v", err)
	}
}

func TestResetToDefaultBranchCheckoutFails(t *testing.T) {
	m := &mockDocker{
		execOut: map[string]string{
			"ls-remote --symref": "ref: refs/heads/main\tHEAD\n",
		},
		execErrs: map[string]error{
			"checkout -f -B": fmt.Errorf("checkout failed"),
		},
	}
	mgr := NewManager(m, "/templates")

	err := mgr.resetToDefaultBranch("test-sandbox")
	if err == nil {
		t.Fatal("expected error when checkout fails")
	}
	if !strings.Contains(err.Error(), "resetting to origin/main") {
		t.Errorf("error should mention resetting to origin/main, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestResetToDefaultBranch -v`
Expected: PASS — all four error cases return the correctly-wrapped error.

- [ ] **Step 3: Commit**

```bash
cd /home/agent/workspace
git add internal/sandbox/sandbox_test.go
git commit -m "test: cover resetToDefaultBranch error paths

ls-remote failure, malformed ls-remote output, fetch failure, and
force-checkout failure each surface a correctly-wrapped error."
```

---

### Task 5: Wire `resetToDefaultBranch` into `Create()`

**Files:**
- Modify: `internal/sandbox/sandbox.go:87-92` (the clean + checkout block inside `Create()`)
- Modify: `internal/sandbox/sandbox_test.go` (update existing `TestCreate` and `TestCreateFailsOnGitSetup` to seed ls-remote output)

- [ ] **Step 1: Seed ls-remote output in existing `TestCreate`**

In `internal/sandbox/sandbox_test.go`, locate `TestCreate` (around line 130). Replace the `m := &mockDocker{}` line with:

```go
	m := &mockDocker{
		execOut: map[string]string{
			"ls-remote --symref": "ref: refs/heads/main\tHEAD\n",
		},
	}
```

Do the same for `TestCreateCallsRewriteHostPaths` (around line 281) — replace its `m := &mockDocker{}` with the identical seeded mock.

Leave `TestCreateFailsOnExecWithStdin`, `TestCreateFailsOnSandboxCreate`, and `TestCreateFailsOnGitSetup` **unseeded**: those tests expect failure and don't reach `resetToDefaultBranch` (or if they do, the failure is the thing being asserted).

- [ ] **Step 2: Run tests to verify existing `TestCreate` fails before wiring**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -run TestCreate -v`
Expected: `TestCreate` and `TestCreateCallsRewriteHostPaths` still PASS (the new seed doesn't change behavior until we wire in the call). This step is the setup — the *next* step drives the behavior change.

- [ ] **Step 3: Replace the clean + checkout block in `Create()`**

In `internal/sandbox/sandbox.go`, replace lines 87-92 (the two `git clean` and `git checkout -b` calls inside `Create()`):

```go
	if _, err := m.docker.SandboxExec(sandboxName, "git", "-C", SandboxWorkspace, "clean", "-fdx", "-q"); err != nil {
		return fmt.Errorf("cleaning workspace: %w", err)
	}
	if _, err := m.docker.SandboxExec(sandboxName, "git", "-C", SandboxWorkspace, "checkout", "-b", opts.SessionID); err != nil {
		return fmt.Errorf("creating session branch: %w", err)
	}
```

with:

```go
	if err := m.resetToDefaultBranch(sandboxName); err != nil {
		return err
	}
	if _, err := m.docker.SandboxExec(sandboxName, "git", "-C", SandboxWorkspace, "checkout", "-b", opts.SessionID); err != nil {
		return fmt.Errorf("creating session branch: %w", err)
	}
```

(`git clean -fdx -q` moves from `Create()` into `resetToDefaultBranch`; the session-branch checkout stays in `Create()`.)

- [ ] **Step 4: Run all unit tests**

Run: `cd /home/agent/workspace && go test ./internal/sandbox/ -v`
Expected: PASS — all tests, including the updated `TestCreate` (which asserts `hasClean` and `hasCheckout` — both still present: clean inside `resetToDefaultBranch`, checkout from both `-f -B` and `-b` calls).

- [ ] **Step 5: Run the full test suite**

Run: `cd /home/agent/workspace && make test`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
cd /home/agent/workspace
git add internal/sandbox/sandbox.go internal/sandbox/sandbox_test.go
git commit -m "feat: reset sandbox to origin/<default> on create

Replaces the tar-piped HEAD-based session-branch origin with a
force-reset to origin/<default>. Resume is untouched."
```

---

### Task 6: Add `createTestWorkspaceWithBareOrigin` integration helper

**Files:**
- Modify: `tests/integration/helpers_test.go`

- [ ] **Step 1: Add the helper**

Append to `tests/integration/helpers_test.go` (after `createTestWorkspace`):

```go
// createTestWorkspaceWithBareOrigin builds a git workspace wired up to a local
// bare repo as "origin". The bare lives inside .git/ so it's carried into the
// sandbox by tar-pipe and survives git clean -fdx. The origin URL uses the
// sandbox-side path and is only read from inside the sandbox; the host pushes
// to the bare via its absolute host path directly.
func createTestWorkspaceWithBareOrigin(t *testing.T, dirname string) string {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), dirname)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	runIn := func(dir string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git -C %s %v failed: %s", dir, args, out)
		}
	}
	runBare := func(args ...string) {
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s", args, out)
		}
	}

	// Initialize workspace on main.
	runIn(workspace, "init", "-q", "--initial-branch=main")
	runIn(workspace, "config", "user.email", "test@example.com")
	runIn(workspace, "config", "user.name", "Test")

	// Create bare repo inside .git/ — never touched by git clean.
	barePath := filepath.Join(workspace, ".git", "integration-test-origin.git")
	runBare("init", "--bare", "-q", "--initial-branch=main", barePath)

	if err := os.WriteFile(filepath.Join(workspace, "testfile.txt"), []byte("test content"), 0o644); err != nil {
		t.Fatalf("write testfile.txt: %v", err)
	}
	runIn(workspace, "add", ".")
	runIn(workspace, "commit", "-q", "-m", "init")

	// Origin URL uses the sandbox path; consumed from inside the sandbox only.
	sandboxOriginURL := "file://" + sandbox.SandboxWorkspace + "/.git/integration-test-origin.git"
	runIn(workspace, "remote", "add", "origin", sandboxOriginURL)

	// Push initial commit to the bare via its host-side absolute path.
	runIn(workspace, "push", "-q", "file://"+barePath, "main:refs/heads/main")

	return workspace
}
```

- [ ] **Step 2: Verify the integration package still compiles**

Run: `cd /home/agent/workspace && go test -tags=integration -run=^NO-SUCH-TEST$ ./tests/integration/...`
Expected: no build errors (no tests match the filter, so nothing runs).

- [ ] **Step 3: Commit**

```bash
cd /home/agent/workspace
git add tests/integration/helpers_test.go
git commit -m "test: add createTestWorkspaceWithBareOrigin integration helper

Builds a workspace with a local bare repo as origin, placed inside .git/
so it's tar-piped into the sandbox and not removed by git clean -fdx."
```

---

### Task 7: Migrate existing integration tests to use `createTestWorkspaceWithBareOrigin`

**Files:**
- Modify: `tests/integration/create_test.go:15`
- Modify: `tests/integration/filesystem_test.go:14`
- Modify: `tests/integration/network_test.go:14` and `:39`
- Modify: `tests/integration/security_test.go:30`

Rationale: after Task 5, `Create()` aborts when there's no `origin`. All existing integration tests that create a sandbox need an origin.

- [ ] **Step 1: Update `create_test.go`**

At `tests/integration/create_test.go:15`, change:

```go
	workspace := createTestWorkspace(t, "cb-create-test")
```

to:

```go
	workspace := createTestWorkspaceWithBareOrigin(t, "cb-create-test")
```

- [ ] **Step 2: Update `filesystem_test.go`**

At `tests/integration/filesystem_test.go:14`, change:

```go
	workspace := createTestWorkspace(t, "cb-fs-test")
```

to:

```go
	workspace := createTestWorkspaceWithBareOrigin(t, "cb-fs-test")
```

- [ ] **Step 3: Update `network_test.go`**

At `tests/integration/network_test.go:14`, change:

```go
	workspace := createTestWorkspace(t, "cb-net-test")
```

to:

```go
	workspace := createTestWorkspaceWithBareOrigin(t, "cb-net-test")
```

At `tests/integration/network_test.go:39`, change:

```go
	workspace := createTestWorkspace(t, "cb-nofilt-test")
```

to:

```go
	workspace := createTestWorkspaceWithBareOrigin(t, "cb-nofilt-test")
```

- [ ] **Step 4: Update `security_test.go`**

At `tests/integration/security_test.go:30`, change:

```go
	workspace := createTestWorkspace(t, "cb-security-test")
```

to:

```go
	workspace := createTestWorkspaceWithBareOrigin(t, "cb-security-test")
```

Leave `createTestWorkspace` itself in place (unused by default now, but we'll re-use it for the no-origin abort test in Task 9).

- [ ] **Step 5: Run integration tests**

Run: `cd /home/agent/workspace && make test-integration`
Expected: PASS (Docker Desktop with sandbox support required).

If Docker is not available locally, note in the commit that integration tests were not locally verified and rely on CI.

- [ ] **Step 6: Commit**

```bash
cd /home/agent/workspace
git add tests/integration/create_test.go tests/integration/filesystem_test.go tests/integration/network_test.go tests/integration/security_test.go
git commit -m "test: migrate integration tests to bare-origin workspace helper

Required because create now aborts without an origin remote."
```

---

### Task 8: Add happy-path integration test (session branch matches origin/main)

**Files:**
- Modify: `tests/integration/filesystem_test.go` (add a subtest to `TestFilesystemLayout`)

- [ ] **Step 1: Add the assertion inside `TestFilesystemLayout`**

In `tests/integration/filesystem_test.go`, add this subtest inside `TestFilesystemLayout` (any position is fine; put it near the "git branch matches sandbox ID" subtest for topical grouping):

```go
	t.Run("session branch matches origin/main tip", func(t *testing.T) {
		sessionTip, err := testDocker.SandboxExec(sb.name, "git", "-C", sandbox.SandboxWorkspace,
			"rev-parse", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		originTip, err := testDocker.SandboxExec(sb.name, "git", "-C", sandbox.SandboxWorkspace,
			"rev-parse", "origin/main")
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(sessionTip) != strings.TrimSpace(originTip) {
			t.Errorf("session HEAD %q != origin/main %q",
				strings.TrimSpace(sessionTip), strings.TrimSpace(originTip))
		}
	})
```

(`strings` is already imported in this file.)

- [ ] **Step 2: Run integration tests**

Run: `cd /home/agent/workspace && make test-integration`
Expected: PASS — the new subtest under `TestFilesystemLayout` asserts the session branch's tip is the same commit as `origin/main`.

- [ ] **Step 3: Commit**

```bash
cd /home/agent/workspace
git add tests/integration/filesystem_test.go
git commit -m "test: verify session branch matches origin/main tip on create"
```

---

### Task 9: Add no-origin abort integration test

**Files:**
- Modify: `tests/integration/filesystem_test.go` (add new top-level test function)

- [ ] **Step 1: Add the abort test**

Append to `tests/integration/filesystem_test.go` (outside `TestFilesystemLayout`, after its closing brace):

```go
func TestCreateAbortsWithoutOrigin(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-no-origin-test") // no remote configured
	buildTemplateImage(t, "jvm")

	sandboxID := sandbox.GenerateSandboxID("jvm")
	name := sandbox.GenerateSandboxName(workspace, sandboxID)
	defer cleanupSandbox(t, name)

	err := testManager.Create(name, sandbox.CreateOpts{
		ImageName: "jvm-sandbox",
		Workspace: workspace,
		ClaudeDir: os.Getenv("HOME") + "/.claude",
		SessionID: sandboxID,
	})
	if err == nil {
		t.Fatal("expected Create to abort when workspace has no origin remote")
	}
	if !strings.Contains(err.Error(), "determining default branch") {
		t.Errorf("error should mention default branch discovery, got: %v", err)
	}
}
```

- [ ] **Step 2: Run integration tests**

Run: `cd /home/agent/workspace && make test-integration`
Expected: PASS — the new test confirms abort happens and surfaces the expected error wrapping.

- [ ] **Step 3: Commit**

```bash
cd /home/agent/workspace
git add tests/integration/filesystem_test.go
git commit -m "test: verify create aborts when workspace has no origin"
```

---

## Final Verification

- [ ] **Step 1: All unit tests pass**

Run: `cd /home/agent/workspace && make test`
Expected: PASS

- [ ] **Step 2: All integration tests pass**

Run: `cd /home/agent/workspace && make test-integration`
Expected: PASS

- [ ] **Step 3: Full suite passes**

Run: `cd /home/agent/workspace && make test-all`
Expected: PASS
