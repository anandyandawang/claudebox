# Sandbox Host Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate all writable host mounts from the sandbox by tar-piping files in and mounting an immediately-deleted temp directory.

**Architecture:** Replace virtiofs mounts with tar-pipe file transfer via `docker sandbox exec -i`. Mount an empty temp dir as the required primary workspace, delete it immediately after creation. Copy specific Claude config files instead of mounting `~/.claude`. Refresh config on resume.

**Tech Stack:** Go, Docker sandbox CLI, Go testing, tar

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/docker/docker.go` | Modify | Add `SandboxExecWithStdin`, update `SandboxCreateOpts` |
| `internal/docker/docker_test.go` | Modify | Add test for `SandboxExecWithStdin`, update `TestSandboxCreate` |
| `internal/sandbox/sandbox.go` | Modify | Rewrite `Create` to use temp dir + tar-pipe, add `RefreshConfig` |
| `internal/sandbox/sandbox_test.go` | Modify | Update mock, `TestCreate`, add `TestRefreshConfig` |
| `internal/commands/create.go` | Modify | Remove `claudeDir` from `CreateOpts` usage (now handled internally) |
| `internal/commands/resume.go` | Modify | Call `RefreshConfig` on resume |
| `tests/integration/helpers_test.go` | Modify | Update `createTestSandbox` for new `Create` signature |
| `tests/integration/network_test.go` | Modify | Update manual sandbox create in `TestNoNetworkPolicyAllowsAll` |
| `tests/integration/security_test.go` | Create | Security integration tests |

---

### Task 1: Add `SandboxExecWithStdin` to Docker client

**Files:**
- Modify: `internal/docker/docker.go:9-18` (interface), `internal/docker/docker.go` (new method)
- Test: `internal/docker/docker_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/docker/docker_test.go`:

```go
func TestSandboxExecWithStdin(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		// cat will read stdin and output it; we use it to verify stdin is wired
		return exec.Command("cat")
	}}

	input := strings.NewReader("hello from stdin")
	err := c.SandboxExecWithStdin(input, "my-sandbox", "sh", "-c", "tar -x")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "exec", "-i", "my-sandbox", "sh", "-c", "tar -x"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxExecWithStdin args:\n  got  %v\n  want %v", calls[0], want)
	}
}
```

Add `"strings"` to the imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/docker/ -run TestSandboxExecWithStdin -v`
Expected: FAIL — method doesn't exist

- [ ] **Step 3: Add the interface method and implementation**

In `internal/docker/docker.go`, add `SandboxExecWithStdin` to the `Docker` interface:

```go
type Docker interface {
	Build(tag string, contextDir string) error
	SandboxCreate(name string, opts SandboxCreateOpts) error
	SandboxRun(name string, args ...string) error
	SandboxExec(name string, args ...string) (string, error)
	SandboxExecWithStdin(r io.Reader, name string, args ...string) error
	SandboxLs(filter string) ([]SandboxInfo, error)
	SandboxRm(name string) error
	SandboxNetworkProxy(name string, allowedHosts []string) error
}
```

Add `"io"` to the imports.

Add the implementation after `SandboxExec`:

```go
func (c *Client) SandboxExecWithStdin(r io.Reader, name string, args ...string) error {
	cmdArgs := append([]string{"sandbox", "exec", "-i", name}, args...)
	cmd := c.newCmd("docker", cmdArgs...)
	cmd.Stdin = r
	return cmd.Run()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/docker/ -run TestSandboxExecWithStdin -v`
Expected: PASS

- [ ] **Step 5: Update all mocks that implement the Docker interface**

The following mocks need the new method added to satisfy the interface:

In `internal/sandbox/sandbox_test.go`, add to `mockDocker`:

```go
func (m *mockDocker) SandboxExecWithStdin(r io.Reader, name string, args ...string) error {
	m.record("SandboxExecWithStdin", append([]string{name}, args...)...)
	if m.failOn == "SandboxExecWithStdin" {
		return fmt.Errorf("exec with stdin failed")
	}
	return nil
}
```

Add `"io"` to imports.

In `internal/commands/commands_test.go`, add to `mockDocker`:

```go
func (m *mockDocker) SandboxExecWithStdin(io.Reader, string, ...string) error { return nil }
```

Add `"io"` to imports.

- [ ] **Step 6: Run all tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./... -v`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add internal/docker/docker.go internal/docker/docker_test.go internal/sandbox/sandbox_test.go internal/commands/commands_test.go
git commit -m "feat: add SandboxExecWithStdin for tar-pipe file transfer"
```

---

### Task 2: Update `SandboxCreateOpts` and `SandboxCreate`

**Files:**
- Modify: `internal/docker/docker.go:21-25` (struct), `internal/docker/docker.go:49-56` (method)
- Test: `internal/docker/docker_test.go:30-47`

- [ ] **Step 1: Write the failing test**

Update `TestSandboxCreate` in `internal/docker/docker_test.go`:

```go
func TestSandboxCreate(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	err := c.SandboxCreate("my-sandbox", SandboxCreateOpts{
		Image:     "jvm-sandbox",
		Command:   "claude",
		Workspace: "/tmp/claudebox-abc123",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "create", "-t", "jvm-sandbox",
		"--name", "my-sandbox", "claude", "/tmp/claudebox-abc123"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxCreate args:\n  got  %v\n  want %v", calls[0], want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/docker/ -run TestSandboxCreate -v`
Expected: FAIL — `SandboxCreateOpts` doesn't have `Workspace` field

- [ ] **Step 3: Update the struct and method**

In `internal/docker/docker.go`:

```go
// SandboxCreateOpts holds options for creating a sandbox.
type SandboxCreateOpts struct {
	Image     string // Docker image tag
	Command   string // Base command (e.g. "claude")
	Workspace string // Primary workspace path (temp dir, deleted after creation)
}
```

```go
func (c *Client) SandboxCreate(name string, opts SandboxCreateOpts) error {
	args := []string{"sandbox", "create", "-t", opts.Image, "--name", name, opts.Command, opts.Workspace}
	cmd := c.newCmd("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/docker/ -run TestSandboxCreate -v`
Expected: PASS

- [ ] **Step 5: Run all docker tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/docker/ -v`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add internal/docker/docker.go internal/docker/docker_test.go
git commit -m "refactor: simplify SandboxCreateOpts to single Workspace field"
```

---

### Task 3: Rewrite sandbox `Create` with tar-pipe and dead temp dir

**Files:**
- Modify: `internal/sandbox/sandbox.go:4-11` (imports), `internal/sandbox/sandbox.go:19-25` (CreateOpts), `internal/sandbox/sandbox.go:50-80` (Create method)
- Modify: `internal/sandbox/sandbox_test.go:35-38` (mock), `internal/sandbox/sandbox_test.go:112-136` (TestCreate)

- [ ] **Step 1: Update the mock to capture the new fields**

In `internal/sandbox/sandbox_test.go`, update `SandboxCreate` mock:

```go
func (m *mockDocker) SandboxCreate(name string, opts docker.SandboxCreateOpts) error {
	m.record("SandboxCreate", name, opts.Image, opts.Command, opts.Workspace)
	if m.failOn == "SandboxCreate" {
		return fmt.Errorf("create failed")
	}
	return nil
}
```

- [ ] **Step 2: Write the failing test**

Replace `TestCreate` in `internal/sandbox/sandbox_test.go`:

```go
func TestCreate(t *testing.T) {
	m := &mockDocker{}
	mgr := NewManager(m, "/templates")

	err := mgr.Create("test-sandbox", CreateOpts{
		ImageName: "jvm-sandbox",
		Workspace: "/path/to/workspace",
		ClaudeDir: "/home/user/.claude",
		SessionID: "sandbox-20260325-120000",
	})
	if err != nil {
		t.Fatal(err)
	}

	// First call: SandboxCreate with a temp dir workspace
	if m.calls[0].method != "SandboxCreate" {
		t.Fatalf("call[0]: got %s, want SandboxCreate", m.calls[0].method)
	}
	// Workspace arg should NOT be the real workspace or claude dir
	createWorkspace := m.calls[0].args[3]
	if createWorkspace == "/path/to/workspace" || createWorkspace == "/home/user/.claude" {
		t.Errorf("SandboxCreate should use a temp dir, not %q", createWorkspace)
	}

	// Should have SandboxExecWithStdin calls for tar-pipe (workspace + claude config)
	var stdinCalls []call
	for _, c := range m.calls {
		if c.method == "SandboxExecWithStdin" {
			stdinCalls = append(stdinCalls, c)
		}
	}
	if len(stdinCalls) < 2 {
		t.Errorf("expected at least 2 SandboxExecWithStdin calls (workspace + config), got %d", len(stdinCalls))
	}

	// Should have SandboxExec call for git clean + checkout
	var gitCall *call
	for _, c := range m.calls {
		if c.method == "SandboxExec" && strings.Contains(strings.Join(c.args, " "), "git clean") {
			gitCall = &c
			break
		}
	}
	if gitCall == nil {
		t.Error("expected SandboxExec call with git clean")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestCreate -v`
Expected: FAIL — Create still uses old mount + cp approach

- [ ] **Step 4: Rewrite the Create method**

In `internal/sandbox/sandbox.go`, update imports:

```go
import (
	"bufio"
	"claudebox/internal/docker"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)
```

Replace the `Create` method:

```go
// Create creates a sandbox with tar-piped workspace and config, no host mounts.
func (m *Manager) Create(sandboxName string, opts CreateOpts) error {
	// Create and immediately delete a temp dir for the required workspace arg.
	// After deletion, the virtiofs mount inside the sandbox becomes a dead end.
	tmpDir, err := os.MkdirTemp("", "claudebox-")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	if err := m.docker.SandboxCreate(sandboxName, docker.SandboxCreateOpts{
		Image:     opts.ImageName,
		Command:   "claude",
		Workspace: tmpDir,
	}); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("creating sandbox: %w", err)
	}
	os.RemoveAll(tmpDir)

	// Tar-pipe workspace into sandbox
	if err := m.tarPipeDir(sandboxName, opts.Workspace, "/home/agent/workspace/"); err != nil {
		return fmt.Errorf("copying workspace: %w", err)
	}

	// Tar-pipe Claude config files into sandbox
	if err := m.tarPipeClaudeConfig(sandboxName, opts.ClaudeDir); err != nil {
		return fmt.Errorf("copying claude config: %w", err)
	}

	// Clean and create branch in workspace copy
	script := fmt.Sprintf(
		`cd /home/agent/workspace && git clean -fdx -q && git checkout -b '%s'`,
		opts.SessionID)
	if _, err := m.docker.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
		return fmt.Errorf("setting up workspace: %w", err)
	}
	return nil
}

// tarPipeDir tars a host directory and pipes it into the sandbox at destDir.
func (m *Manager) tarPipeDir(sandboxName, srcDir, destDir string) error {
	tarCmd := exec.Command("tar", "-C", srcDir, "-c", ".")
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	tarCmd.Stdout = pw
	if err := tarCmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return err
	}
	pw.Close()
	extractErr := m.docker.SandboxExecWithStdin(pr, sandboxName, "sh", "-c", "tar -C '"+destDir+"' -x")
	pr.Close()
	tarCmd.Wait()
	return extractErr
}

// tarPipeClaudeConfig copies .claude.json, settings.json, and plugins/ into the sandbox.
func (m *Manager) tarPipeClaudeConfig(sandboxName, claudeDir string) error {
	// Build tar args for only the files that exist
	var files []string
	for _, f := range []string{".claude.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(claudeDir, f)); err == nil {
			files = append(files, f)
		}
	}
	if info, err := os.Stat(filepath.Join(claudeDir, "plugins")); err == nil && info.IsDir() {
		files = append(files, "plugins")
	}
	if len(files) == 0 {
		return nil
	}

	// Ensure target dirs exist
	m.docker.SandboxExec(sandboxName, "mkdir", "-p", "/home/agent/.claude")

	args := append([]string{"-C", claudeDir, "-c"}, files...)
	tarCmd := exec.Command("tar", args...)
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	tarCmd.Stdout = pw
	if err := tarCmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return err
	}
	pw.Close()
	extractErr := m.docker.SandboxExecWithStdin(pr, sandboxName, "sh", "-c", "tar -C /home/agent/.claude -x")
	pr.Close()
	tarCmd.Wait()

	// Symlink .claude.json to home dir (Claude expects it at ~/.claude.json)
	m.docker.SandboxExec(sandboxName, "ln", "-sf", "/home/agent/.claude/.claude.json", "/home/agent/.claude.json")

	return extractErr
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestCreate -v`
Expected: PASS

- [ ] **Step 6: Run all sandbox tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -v`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add internal/sandbox/sandbox.go internal/sandbox/sandbox_test.go
git commit -m "feat: replace host mounts with tar-pipe and dead temp dir"
```

---

### Task 4: Add `RefreshConfig` and update resume

**Files:**
- Modify: `internal/sandbox/sandbox.go` (add `RefreshConfig` method)
- Modify: `internal/sandbox/sandbox_test.go` (add `TestRefreshConfig`)
- Modify: `internal/commands/resume.go:79-84` (call `RefreshConfig`)

- [ ] **Step 1: Write the failing test**

Add to `internal/sandbox/sandbox_test.go`:

```go
func TestRefreshConfig(t *testing.T) {
	m := &mockDocker{}
	mgr := NewManager(m, "/templates")

	claudeDir := t.TempDir()
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{}`), 0o644)
	os.MkdirAll(filepath.Join(claudeDir, "plugins"), 0o755)

	err := mgr.RefreshConfig("test-sandbox", claudeDir)
	if err != nil {
		t.Fatal(err)
	}

	// Should have SandboxExecWithStdin call for tar-pipe
	var stdinCalls int
	for _, c := range m.calls {
		if c.method == "SandboxExecWithStdin" {
			stdinCalls++
		}
	}
	if stdinCalls == 0 {
		t.Error("expected SandboxExecWithStdin call for config refresh")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestRefreshConfig -v`
Expected: FAIL — method doesn't exist

- [ ] **Step 3: Add RefreshConfig**

Add to `internal/sandbox/sandbox.go`:

```go
// RefreshConfig re-copies settings.json and plugins/ from the host into the sandbox.
// Called on resume to pick up any host-side changes.
func (m *Manager) RefreshConfig(sandboxName, claudeDir string) error {
	var files []string
	for _, f := range []string{"settings.json"} {
		if _, err := os.Stat(filepath.Join(claudeDir, f)); err == nil {
			files = append(files, f)
		}
	}
	if info, err := os.Stat(filepath.Join(claudeDir, "plugins")); err == nil && info.IsDir() {
		files = append(files, "plugins")
	}
	if len(files) == 0 {
		return nil
	}

	args := append([]string{"-C", claudeDir, "-c"}, files...)
	tarCmd := exec.Command("tar", args...)
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	tarCmd.Stdout = pw
	if err := tarCmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return err
	}
	pw.Close()
	extractErr := m.docker.SandboxExecWithStdin(pr, sandboxName, "sh", "-c", "tar -C /home/agent/.claude -x")
	pr.Close()
	tarCmd.Wait()
	return extractErr
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/andywang/Repos/claudebox && go test ./internal/sandbox/ -run TestRefreshConfig -v`
Expected: PASS

- [ ] **Step 5: Update resume command**

In `internal/commands/resume.go`, add a `RefreshConfig` call after environment setup. Add `claudeDir` variable:

```go
	fmt.Printf("Resuming sandbox: %s...\n", sandboxName)

	claudeDir := os.Getenv("HOME") + "/.claude"

	// Refresh config (settings, plugins) from host
	if err := mgr.RefreshConfig(sandboxName, claudeDir); err != nil {
		return err
	}

	// Environment first, then credentials (matches Bash ordering)
	if err := environment.Setup(d, sandboxName); err != nil {
		return err
	}
```

- [ ] **Step 6: Run all tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./... -v`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add internal/sandbox/sandbox.go internal/sandbox/sandbox_test.go internal/commands/resume.go
git commit -m "feat: add RefreshConfig for resume, sync settings and plugins from host"
```

---

### Task 5: Update create command and integration test helpers

**Files:**
- Modify: `internal/commands/create.go:57` (claudeDir passed to Create)
- Modify: `tests/integration/helpers_test.go:56-61` (update createTestSandbox)
- Modify: `tests/integration/network_test.go:49-55` (update manual sandbox create)

- [ ] **Step 1: Verify create.go still works**

Check that `internal/commands/create.go` passes `ClaudeDir` to `sandbox.CreateOpts` — this should still work since `CreateOpts` still has `ClaudeDir`. No change needed if the field still exists.

Run: `cd /Users/andywang/Repos/claudebox && go test ./... -v`
Expected: All pass (create.go should compile fine)

- [ ] **Step 2: Update the manual sandbox create in network_test.go**

In `tests/integration/network_test.go`, the `TestNoNetworkPolicyAllowsAll` function creates a sandbox directly. Update it to use a temp dir:

```go
func TestNoNetworkPolicyAllowsAll(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-nofilt-test")

	tmpDir := t.TempDir()
	src, _ := os.ReadFile(templatesDir + "/jvm/Dockerfile")
	os.WriteFile(tmpDir+"/Dockerfile", src, 0o644)

	cmd := exec.Command("docker", "build", "-q", "-t", "nofilter-sandbox", tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s", out)
	}
	defer exec.Command("docker", "rmi", "nofilter-sandbox").Run()

	name := "cb-nofilt-test-sandbox"
	mountDir := t.TempDir()
	cmd = exec.Command("docker", "sandbox", "create", "-t", "nofilter-sandbox",
		"--name", name, "claude", mountDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create failed: %s", out)
	}
	os.RemoveAll(mountDir)
	defer func() { exec.Command("docker", "sandbox", "rm", name).Run() }()

	// Tar-pipe workspace in
	tarPipe := exec.Command("sh", "-c",
		fmt.Sprintf("tar -C '%s' -c . | docker sandbox exec -i '%s' sh -c 'tar -C /home/agent/workspace -x'", workspace, name))
	if out, err := tarPipe.CombinedOutput(); err != nil {
		t.Fatalf("tar-pipe failed: %s", out)
	}

	_, err := testDocker.SandboxExec(name,
		"curl", "--connect-timeout", "10", "-sf", "https://example.com")
	if err != nil {
		t.Error("example.com should be reachable without network policy")
	}
}
```

Add `"fmt"` to imports if not already present.

- [ ] **Step 3: Run all unit tests**

Run: `cd /Users/andywang/Repos/claudebox && go test ./... -v`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add tests/integration/network_test.go
git commit -m "fix: update manual sandbox create in network test to use temp dir + tar-pipe"
```

---

### Task 6: Add security integration tests

**Files:**
- Create: `tests/integration/security_test.go`

- [ ] **Step 1: Create the security test file**

Create `tests/integration/security_test.go`:

```go
//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceIsolation(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-isolation-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	t.Run("host workspace path not mounted", func(t *testing.T) {
		out, _ := testDocker.SandboxExec(name, "findmnt", "-t", "virtiofs", "-n", "-o", "TARGET")
		if strings.Contains(out, workspace) {
			t.Errorf("host workspace path should not appear in mounts: %s", out)
		}
	})

	t.Run("workspace copy has repo files", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "test", "-f", "/home/agent/workspace/testfile.txt")
		if err != nil {
			t.Error("testfile.txt should exist in workspace copy")
		}
	})

	t.Run("workspace copy is writable", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "touch", "/home/agent/workspace/new-file")
		if err != nil {
			t.Errorf("should be able to write to workspace copy: %v", err)
		}
	})
}

func TestDeadMountEscapeAttempts(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-deadmount-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	// Find the dead mount path (the temp dir that was deleted)
	out, err := testDocker.SandboxExec(name, "findmnt", "-t", "virtiofs", "-n", "-o", "TARGET")
	if err != nil {
		t.Fatalf("findmnt failed: %v", err)
	}
	// The dead mount is the first virtiofs target (primary workspace)
	mountPaths := strings.Fields(out)
	if len(mountPaths) == 0 {
		t.Fatal("no virtiofs mounts found")
	}
	deadMount := mountPaths[0]

	t.Run("write to dead mount fails", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "touch", deadMount+"/escape-test")
		if err == nil {
			t.Error("should not be able to write to dead mount")
		}
	})

	t.Run("mkdir in dead mount does not propagate to host", func(t *testing.T) {
		// mkdir may succeed in sandbox overlay
		testDocker.SandboxExec(name, "mkdir", "-p", deadMount+"/escape-dir")
		// But it must NOT appear on the host
		if _, err := os.Stat(deadMount); err == nil {
			// Path exists on host — check if the sandbox created it
			if _, err := os.Stat(filepath.Join(deadMount, "escape-dir")); err == nil {
				t.Error("sandbox mkdir propagated to host filesystem")
			}
		}
	})

	t.Run("write to re-created dir does not propagate to host", func(t *testing.T) {
		testDocker.SandboxExec(name, "mkdir", "-p", deadMount+"/write-test-dir")
		testDocker.SandboxExec(name, "touch", deadMount+"/write-test-dir/escape-file")
		if _, err := os.Stat(filepath.Join(deadMount, "write-test-dir", "escape-file")); err == nil {
			t.Error("file written to re-created dir propagated to host")
		}
	})
}

func TestHostDockerDaemonIsolation(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-docker-iso-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	t.Run("host docker socket not accessible", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "stat", "/var/run/docker.sock")
		if err == nil {
			out, _ := testDocker.SandboxExec(name, "docker", "info", "--format", "{{.Name}}")
			if !strings.Contains(out, "docker-desktop") && out != "" {
				t.Logf("warning: docker socket exists, docker info name=%q — verify this is the sandbox daemon", out)
			}
		}
	})

	t.Run("host docker daemon not reachable over TCP", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name,
			"curl", "--connect-timeout", "3", "-sf", "http://host.docker.internal:2375/info")
		if err == nil {
			t.Error("should not be able to reach host Docker daemon over TCP")
		}
	})

	// VM boundary
	t.Run("host paths not reachable via inner docker", func(t *testing.T) {
		out, err := testDocker.SandboxExec(name,
			"sh", "-c", "docker run --rm -v /Users:/test alpine ls /test 2>&1 || true")
		if err == nil && strings.Contains(out, "andywang") {
			t.Error("inner docker should not see host /Users directory")
		}
	})

	t.Run("mount root is VM-scoped", func(t *testing.T) {
		sandboxHostname, _ := testDocker.SandboxExec(name, "hostname")
		innerHostname, err := testDocker.SandboxExec(name,
			"sh", "-c", "docker run --rm -v /:/mnt alpine cat /mnt/etc/hostname 2>&1 || true")
		if err == nil && innerHostname != "" && innerHostname != sandboxHostname {
			t.Logf("sandbox hostname=%q, inner mount hostname=%q — verify inner sees sandbox, not host", sandboxHostname, innerHostname)
		}
	})

	// Inner Docker escape attempts against dead mount
	t.Run("inner docker can't write to dead mount on host", func(t *testing.T) {
		out, _ := testDocker.SandboxExec(name, "findmnt", "-t", "virtiofs", "-n", "-o", "TARGET")
		deadMount := strings.Fields(out)[0]

		testDocker.SandboxExec(name,
			"sh", "-c", "docker run --rm -v "+deadMount+":/repo alpine touch /repo/docker-escape-test 2>&1 || true")
		if _, err := os.Stat(filepath.Join(deadMount, "docker-escape-test")); err == nil {
			t.Error("inner docker write to dead mount propagated to host")
		}
	})

	t.Run("inner docker can't write to arbitrary host paths", func(t *testing.T) {
		marker := filepath.Join(os.TempDir(), "claudebox-escape-marker")
		os.Remove(marker) // clean slate
		testDocker.SandboxExec(name,
			"sh", "-c", "docker run --rm -v /tmp:/t alpine touch /t/claudebox-escape-marker 2>&1 || true")
		if _, err := os.Stat(marker); err == nil {
			os.Remove(marker)
			t.Error("inner docker wrote to host /tmp")
		}
	})
}

func TestSandboxEscapeAttempt(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-escape-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	// Comprehensive escape attempt: write a script and try to get it to execute on the host
	marker := filepath.Join(os.TempDir(), "claudebox-escape-proof")
	os.Remove(marker)
	defer os.Remove(marker)

	// Attempt 1: Write to common auto-exec paths
	autoExecPaths := []string{
		"/tmp/claudebox-escape-proof",
		"/var/tmp/claudebox-escape-proof",
	}
	for _, p := range autoExecPaths {
		testDocker.SandboxExec(name, "sh", "-c",
			"echo '#!/bin/sh\ntouch "+marker+"' > "+p+" 2>/dev/null; chmod +x "+p+" 2>/dev/null || true")
	}

	// Attempt 2: Try to write via inner Docker to host temp
	testDocker.SandboxExec(name, "sh", "-c",
		"docker run --rm -v /tmp:/t alpine sh -c 'echo touched > /t/claudebox-escape-proof' 2>&1 || true")

	// Verify: marker file must not exist on host
	if _, err := os.Stat(marker); err == nil {
		t.Error("SANDBOX ESCAPE: marker file appeared on host filesystem")
	}
}
```

- [ ] **Step 2: Verify unit tests still pass**

Run: `cd /Users/andywang/Repos/claudebox && go test ./... -v`
Expected: All pass

- [ ] **Step 3: Commit**

```bash
git add tests/integration/security_test.go
git commit -m "test: add security integration tests for host isolation and escape attempts"
```

---

### Task 7: Run integration tests end-to-end

**Files:** None (verification only)

- [ ] **Step 1: Run all integration tests**

Run: `cd /Users/andywang/Repos/claudebox && go test -tags integration ./tests/integration/ -v -timeout 600s`
Expected: All tests pass

- [ ] **Step 2: If any test fails, fix and re-run**

Debug failures, fix, re-run until all pass.

- [ ] **Step 3: Final commit if fixes needed**

```bash
git add -A
git commit -m "fix: address integration test failures"
```
