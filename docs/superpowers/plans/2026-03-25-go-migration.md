# Go Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite claudebox from Bash to idiomatic Go, preserving identical CLI behavior.

**Architecture:** Bottom-up build: docker wrapper -> credentials/environment -> sandbox manager -> cobra commands. The `Docker` interface is the mock boundary for all unit tests. Commands are thin orchestrators calling into focused packages.

**Tech Stack:** Go, cobra (CLI framework), stdlib `os/exec` for docker CLI calls, stdlib `testing` for tests.

**Spec:** `docs/superpowers/specs/2026-03-25-go-migration-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `cmd/claudebox/main.go` | Entry point, cobra root command, template dir resolution |
| `internal/docker/docker.go` | `Docker` interface, `Client` struct, all docker CLI wrappers |
| `internal/docker/docker_test.go` | Verify CLI arg construction for each method |
| `internal/credentials/keychain.go` | Read macOS Keychain, inject into sandbox |
| `internal/credentials/keychain_test.go` | Mock keychain + docker exec |
| `internal/environment/environment.go` | Sandbox env setup (proxy, JVM, GitHub) |
| `internal/environment/environment_test.go` | Mock docker exec calls |
| `internal/sandbox/naming.go` | Workspace name sanitization + sandbox name generation |
| `internal/sandbox/naming_test.go` | Pure function tests |
| `internal/sandbox/sandbox.go` | Sandbox lifecycle: validate, build, create, network, wrap |
| `internal/sandbox/sandbox_test.go` | Mock Docker interface, verify orchestration |
| `internal/commands/ls.go` | `claudebox ls` command |
| `internal/commands/rm.go` | `claudebox rm` command |
| `internal/commands/create.go` | `claudebox <template>` logic |
| `internal/commands/resume.go` | `claudebox resume` command |
| `internal/commands/commands_test.go` | Shared mock + command tests |
| `tests/integration/helpers_test.go` | Shared test infrastructure + TestMain |
| `tests/integration/create_test.go` | Image build + sandbox creation |
| `tests/integration/network_test.go` | Network policy enforcement |
| `tests/integration/filesystem_test.go` | Workspace layout + git branch + config symlinks |

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/claudebox/main.go`

- [ ] **Step 1: Initialize Go module and install cobra**

```bash
cd /Users/andywang/Repos/claudebox
go mod init claudebox
go get github.com/spf13/cobra@latest
```

- [ ] **Step 2: Create main.go with root command and stub subcommands**

```go
// cmd/claudebox/main.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func main() {
	templatesDir, err := findTemplatesDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	_ = templatesDir // used in later tasks

	rootCmd := &cobra.Command{
		Use:   "claudebox [template] [workspace] [-- agent_args...]",
		Short: "Run Claude Code in sandboxed Docker containers",
		Long: `claudebox creates isolated Docker sandbox environments for Claude Code
with per-template toolchains and network restrictions.

Each run creates a new sandbox with a local copy of the repo,
so multiple sessions can work on independent branches in parallel.`,
	}

	rootCmd.AddCommand(
		&cobra.Command{Use: "ls", Short: "List all sandboxes", RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		}},
		&cobra.Command{Use: "rm", Short: "Remove sandboxes", RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		}},
		&cobra.Command{Use: "resume", Short: "Resume an existing sandbox", RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		}},
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// findTemplatesDir resolves the templates directory relative to the binary,
// following symlinks (same behavior as the Bash version).
func findTemplatesDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("cannot resolve symlinks: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "templates"), nil
}
```

- [ ] **Step 3: Verify it builds and shows help**

```bash
go build -o cb-go ./cmd/claudebox && ./cb-go --help
```

Expected: help output showing `ls`, `rm`, `resume` subcommands. Build to `cb-go` to avoid colliding with the Bash `claudebox` script during migration.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum cmd/
git commit -m "feat: scaffold Go project with cobra root command"
```

---

### Task 2: Docker Package (TDD)

**Files:**
- Create: `internal/docker/docker.go`
- Create: `internal/docker/docker_test.go`

- [ ] **Step 1: Write tests for all Client methods**

The `Client` struct has a `newCmd` field (defaults to `exec.Command`) that tests replace to capture args without running docker.

```go
// internal/docker/docker_test.go
package docker

import (
	"os/exec"
	"reflect"
	"testing"
)

// captureCmd records the command args and returns a no-op command.
func captureCmd(calls *[][]string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return exec.Command("true")
	}
}

func TestBuild(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	if err := c.Build("jvm-sandbox", "/path/to/templates/jvm"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "build", "-t", "jvm-sandbox", "/path/to/templates/jvm"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Errorf("Build args:\n  got  %v\n  want %v", calls, want)
	}
}

func TestSandboxCreate(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	err := c.SandboxCreate("my-sandbox", SandboxCreateOpts{
		Image:   "jvm-sandbox",
		Command: "claude",
		Mounts:  []string{"/workspace", "/home/user/.claude"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "create", "-t", "jvm-sandbox",
		"--name", "my-sandbox", "claude", "/workspace", "/home/user/.claude"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxCreate args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxExec(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		return exec.Command("echo", "output")
	}}

	out, err := c.SandboxExec("my-sandbox", "sh", "-c", "echo hello")
	if err != nil {
		t.Fatal(err)
	}
	if out != "output" {
		t.Errorf("output: got %q, want %q", out, "output")
	}
	want := []string{"docker", "sandbox", "exec", "my-sandbox", "sh", "-c", "echo hello"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxExec args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxLs(t *testing.T) {
	c := &Client{newCmd: func(name string, args ...string) *exec.Cmd {
		return exec.Command("printf", "NAME\tSTATUS\nfoo-sandbox\trunning\nbar-sandbox\tstopped\n")
	}}

	sandboxes, err := c.SandboxLs("")
	if err != nil {
		t.Fatal(err)
	}
	if len(sandboxes) != 2 {
		t.Fatalf("count: got %d, want 2", len(sandboxes))
	}
	if sandboxes[0].Name != "foo-sandbox" || sandboxes[1].Name != "bar-sandbox" {
		t.Errorf("names: got %v", sandboxes)
	}
}

func TestSandboxLsWithFilter(t *testing.T) {
	c := &Client{newCmd: func(name string, args ...string) *exec.Cmd {
		return exec.Command("printf", "NAME\tSTATUS\nfoo-sandbox\trunning\nbar-sandbox\tstopped\n")
	}}

	sandboxes, err := c.SandboxLs("foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(sandboxes) != 1 || sandboxes[0].Name != "foo-sandbox" {
		t.Errorf("filtered: got %v, want [foo-sandbox]", sandboxes)
	}
}

func TestSandboxRm(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	if err := c.SandboxRm("my-sandbox"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "rm", "my-sandbox"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxRm args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxRun(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	if err := c.SandboxRun("my-sandbox", "--dangerously-skip-permissions"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "run", "my-sandbox", "--", "--dangerously-skip-permissions"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxRun args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxNetworkProxy(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	err := c.SandboxNetworkProxy("my-sandbox", []string{"api.github.com", "registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "network", "proxy", "my-sandbox",
		"--policy", "deny", "--allow-host", "api.github.com", "--allow-host", "registry.npmjs.org"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxNetworkProxy args:\n  got  %v\n  want %v", calls[0], want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/docker/
```

Expected: compilation error — package doesn't exist.

- [ ] **Step 3: Implement docker.go**

```go
// internal/docker/docker.go
package docker

import (
	"os"
	"os/exec"
	"strings"
)

// Docker defines the interface for docker sandbox operations.
type Docker interface {
	Build(tag string, contextDir string) error
	SandboxCreate(name string, opts SandboxCreateOpts) error
	SandboxRun(name string, args ...string) error
	SandboxExec(name string, args ...string) (string, error)
	SandboxLs(filter string) ([]SandboxInfo, error)
	SandboxRm(name string) error
	SandboxNetworkProxy(name string, allowedHosts []string) error
}

// SandboxCreateOpts holds options for creating a sandbox.
type SandboxCreateOpts struct {
	Image   string   // Docker image tag
	Command string   // Base command (e.g. "claude")
	Mounts  []string // Positional args: workspace path, claude config dir
}

// SandboxInfo represents a sandbox from docker sandbox ls.
type SandboxInfo struct {
	Name string
}

// Client implements Docker by shelling out to the docker CLI.
type Client struct {
	newCmd func(name string, args ...string) *exec.Cmd
}

// NewClient returns a Client that runs real docker commands.
func NewClient() *Client {
	return &Client{newCmd: exec.Command}
}

func (c *Client) Build(tag string, contextDir string) error {
	cmd := c.newCmd("docker", "build", "-t", tag, contextDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Client) SandboxCreate(name string, opts SandboxCreateOpts) error {
	args := []string{"sandbox", "create", "-t", opts.Image, "--name", name, opts.Command}
	args = append(args, opts.Mounts...)
	cmd := c.newCmd("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Client) SandboxRun(name string, args ...string) error {
	cmdArgs := []string{"sandbox", "run", name}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, "--")
		cmdArgs = append(cmdArgs, args...)
	}
	cmd := c.newCmd("docker", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Client) SandboxExec(name string, args ...string) (string, error) {
	cmdArgs := append([]string{"sandbox", "exec", name}, args...)
	cmd := c.newCmd("docker", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) SandboxLs(filter string) ([]SandboxInfo, error) {
	cmd := c.newCmd("docker", "sandbox", "ls")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var sandboxes []SandboxInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		if i == 0 {
			continue // skip header
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if filter != "" && !strings.HasPrefix(name, filter) {
			continue
		}
		sandboxes = append(sandboxes, SandboxInfo{Name: name})
	}
	return sandboxes, nil
}

func (c *Client) SandboxRm(name string) error {
	cmd := c.newCmd("docker", "sandbox", "rm", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Client) SandboxNetworkProxy(name string, allowedHosts []string) error {
	args := []string{"sandbox", "network", "proxy", name, "--policy", "deny"}
	for _, host := range allowedHosts {
		args = append(args, "--allow-host", host)
	}
	cmd := c.newCmd("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/docker/ -v
```

Expected: all 8 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/docker/
git commit -m "feat: add Docker CLI wrapper with interface and client"
```

---

### Task 3: Sandbox Naming (TDD)

**Files:**
- Create: `internal/sandbox/naming.go`
- Create: `internal/sandbox/naming_test.go`

- [ ] **Step 1: Write tests for name sanitization and generation**

```go
// internal/sandbox/naming_test.go
package sandbox

import (
	"regexp"
	"testing"
)

func TestSanitizeWorkspaceName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"my-project", "my-project"},
		{"my project", "my-project"},
		{"project@v2!", "project-v2-"},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		got := SanitizeWorkspaceName(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeWorkspaceName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateSandboxName(t *testing.T) {
	name := GenerateSandboxName("/path/to/my-project", "jvm")
	pattern := `^my-project-jvm-sandbox-\d{8}-\d{6}$`
	if matched, _ := regexp.MatchString(pattern, name); !matched {
		t.Errorf("GenerateSandboxName = %q, want match %s", name, pattern)
	}
}

func TestGenerateSessionID(t *testing.T) {
	id := GenerateSessionID()
	pattern := `^sandbox-\d{8}-\d{6}$`
	if matched, _ := regexp.MatchString(pattern, id); !matched {
		t.Errorf("GenerateSessionID = %q, want match %s", id, pattern)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/sandbox/
```

- [ ] **Step 3: Implement naming.go**

```go
// internal/sandbox/naming.go
package sandbox

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_.\-]+`)

// SanitizeWorkspaceName replaces non-alphanumeric chars (except _ . -) with hyphens.
// Matches the Bash: tr -cs 'a-zA-Z0-9_.-' '-'
func SanitizeWorkspaceName(name string) string {
	return nonAlphanumeric.ReplaceAllString(name, "-")
}

// GenerateSessionID returns a session ID: sandbox-YYYYMMDD-HHMMSS.
func GenerateSessionID() string {
	return fmt.Sprintf("sandbox-%s", time.Now().Format("20060102-150405"))
}

// GenerateSandboxName returns: <workspace>-<template>-sandbox-YYYYMMDD-HHMMSS.
func GenerateSandboxName(workspacePath, template string) string {
	wsName := SanitizeWorkspaceName(filepath.Base(workspacePath))
	return fmt.Sprintf("%s-%s-%s", wsName, template, GenerateSessionID())
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/sandbox/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/sandbox/
git commit -m "feat: add sandbox naming utilities"
```

---

### Task 4: Sandbox Manager (TDD)

**Files:**
- Create: `internal/sandbox/sandbox.go`
- Create: `internal/sandbox/sandbox_test.go`

- [ ] **Step 1: Write tests with mock Docker**

Create a mock that records all calls to verify orchestration:

```go
// internal/sandbox/sandbox_test.go
package sandbox

import (
	"claudebox/internal/docker"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type call struct {
	method string
	args   []string
}

type mockDocker struct {
	calls    []call
	execOut  map[string]string // substring match in args -> return value
	lsOutput []docker.SandboxInfo
	failOn   string
}

func (m *mockDocker) record(method string, args ...string) {
	m.calls = append(m.calls, call{method, args})
}

func (m *mockDocker) Build(tag, contextDir string) error {
	m.record("Build", tag, contextDir)
	if m.failOn == "Build" { return fmt.Errorf("build failed") }
	return nil
}

func (m *mockDocker) SandboxCreate(name string, opts docker.SandboxCreateOpts) error {
	m.record("SandboxCreate", name, opts.Image, opts.Command)
	if m.failOn == "SandboxCreate" { return fmt.Errorf("create failed") }
	return nil
}

func (m *mockDocker) SandboxRun(name string, args ...string) error {
	m.record("SandboxRun", append([]string{name}, args...)...)
	return nil
}

func (m *mockDocker) SandboxExec(name string, args ...string) (string, error) {
	m.record("SandboxExec", append([]string{name}, args...)...)
	if m.failOn == "SandboxExec" { return "", fmt.Errorf("exec failed") }
	for prefix, out := range m.execOut {
		if strings.Contains(strings.Join(args, " "), prefix) {
			return out, nil
		}
	}
	return "", nil
}

func (m *mockDocker) SandboxLs(filter string) ([]docker.SandboxInfo, error) {
	m.record("SandboxLs", filter)
	if filter == "" { return m.lsOutput, nil }
	var filtered []docker.SandboxInfo
	for _, s := range m.lsOutput {
		if strings.HasPrefix(s.Name, filter) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (m *mockDocker) SandboxRm(name string) error {
	m.record("SandboxRm", name)
	return nil
}

func (m *mockDocker) SandboxNetworkProxy(name string, hosts []string) error {
	m.record("SandboxNetworkProxy", append([]string{name}, hosts...)...)
	return nil
}

// --- Tests ---

func TestValidateTemplate(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "jvm"), 0o755)
	os.WriteFile(filepath.Join(dir, "jvm", "Dockerfile"), []byte("FROM scratch"), 0o644)

	mgr := NewManager(&mockDocker{}, dir)

	if err := mgr.ValidateTemplate("jvm"); err != nil {
		t.Errorf("ValidateTemplate(jvm) should pass: %v", err)
	}
	if err := mgr.ValidateTemplate("nonexistent"); err == nil {
		t.Error("ValidateTemplate(nonexistent) should fail")
	}
}

func TestBuildImage(t *testing.T) {
	m := &mockDocker{}
	mgr := NewManager(m, "/templates")

	imageName, err := mgr.BuildImage("jvm")
	if err != nil {
		t.Fatal(err)
	}
	if imageName != "jvm-sandbox" {
		t.Errorf("BuildImage: got %q, want %q", imageName, "jvm-sandbox")
	}
	if m.calls[0].method != "Build" || m.calls[0].args[0] != "jvm-sandbox" {
		t.Errorf("BuildImage call: got %v", m.calls[0])
	}
}

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

	// First call: SandboxCreate
	if m.calls[0].method != "SandboxCreate" {
		t.Errorf("call[0]: got %s, want SandboxCreate", m.calls[0].method)
	}
	// Remaining calls: SandboxExec for symlinks and workspace copy
	for _, c := range m.calls[1:] {
		if c.method != "SandboxExec" {
			t.Errorf("unexpected call: %s", c.method)
		}
	}
}

func TestWrapClaudeBinary(t *testing.T) {
	m := &mockDocker{}
	mgr := NewManager(m, "/templates")

	if err := mgr.WrapClaudeBinary("my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 1 || m.calls[0].method != "SandboxExec" {
		t.Errorf("WrapClaudeBinary: got %v", m.calls)
	}
	// Verify the script contains the cd wrapper
	script := strings.Join(m.calls[0].args, " ")
	if !strings.Contains(script, "claude-real") {
		t.Error("WrapClaudeBinary script should reference claude-real")
	}
}

func TestApplyNetworkPolicy(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "jvm"), 0o755)
	os.WriteFile(filepath.Join(dir, "jvm", "allowed-hosts.txt"),
		[]byte("api.github.com\n# comment\n\nregistry.npmjs.org\n"), 0o644)

	m := &mockDocker{}
	mgr := NewManager(m, dir)

	applied, err := mgr.ApplyNetworkPolicy("my-sandbox", "jvm")
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Error("should return true when hosts file exists")
	}
	if m.calls[0].method != "SandboxNetworkProxy" {
		t.Errorf("call: got %s, want SandboxNetworkProxy", m.calls[0].method)
	}
	// 2 hosts (comment and blank line skipped)
	hosts := m.calls[0].args[1:]
	if len(hosts) != 2 || hosts[0] != "api.github.com" || hosts[1] != "registry.npmjs.org" {
		t.Errorf("hosts: got %v", hosts)
	}
}

func TestApplyNetworkPolicyNoFile(t *testing.T) {
	m := &mockDocker{}
	mgr := NewManager(m, t.TempDir())

	applied, err := mgr.ApplyNetworkPolicy("my-sandbox", "jvm")
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Error("should return false when no hosts file")
	}
}

func TestVerifyNetworkPolicy(t *testing.T) {
	m := &mockDocker{}
	mgr := NewManager(m, "/templates")

	// VerifyNetworkPolicy calls SandboxExec twice (curl blocked + curl allowed)
	_ = mgr.VerifyNetworkPolicy("my-sandbox")
	if len(m.calls) != 2 {
		t.Errorf("expected 2 exec calls, got %d", len(m.calls))
	}
}

func TestList(t *testing.T) {
	m := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: "proj-jvm-sandbox-1"},
		{Name: "other-sandbox-2"},
	}}
	mgr := NewManager(m, "/templates")

	names, err := mgr.List("proj-")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "proj-jvm-sandbox-1" {
		t.Errorf("List: got %v", names)
	}
}

func TestRemoveAll(t *testing.T) {
	m := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: "proj-jvm-1"},
		{Name: "proj-jvm-2"},
		{Name: "other-sandbox"},
	}}
	mgr := NewManager(m, "/templates")

	count, err := mgr.RemoveAll("proj-")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("RemoveAll: got %d, want 2", count)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/sandbox/
```

Expected: compilation error — `Manager`, `CreateOpts`, etc. undefined.

- [ ] **Step 3: Implement sandbox.go**

```go
// internal/sandbox/sandbox.go
package sandbox

import (
	"bufio"
	"claudebox/internal/docker"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Manager handles sandbox lifecycle operations.
type Manager struct {
	docker       docker.Docker
	templatesDir string
}

// CreateOpts holds options for creating a sandbox.
type CreateOpts struct {
	ImageName string
	Workspace string
	ClaudeDir string
	SessionID string
}

// NewManager returns a Manager.
func NewManager(d docker.Docker, templatesDir string) *Manager {
	return &Manager{docker: d, templatesDir: templatesDir}
}

// ValidateTemplate checks the template directory contains a Dockerfile.
func (m *Manager) ValidateTemplate(template string) error {
	df := filepath.Join(m.templatesDir, template, "Dockerfile")
	if _, err := os.Stat(df); err != nil {
		return fmt.Errorf("no Dockerfile found in %s", filepath.Join(m.templatesDir, template))
	}
	return nil
}

// BuildImage builds the Docker image for a template. Returns image name.
func (m *Manager) BuildImage(template string) (string, error) {
	imageName := template + "-sandbox"
	if err := m.docker.Build(imageName, filepath.Join(m.templatesDir, template)); err != nil {
		return "", fmt.Errorf("building image: %w", err)
	}
	return imageName, nil
}

// Create creates a sandbox, symlinks config, copies workspace, and creates a git branch.
func (m *Manager) Create(sandboxName string, opts CreateOpts) error {
	if err := m.docker.SandboxCreate(sandboxName, docker.SandboxCreateOpts{
		Image:   opts.ImageName,
		Command: "claude",
		Mounts:  []string{opts.Workspace, opts.ClaudeDir},
	}); err != nil {
		return fmt.Errorf("creating sandbox: %w", err)
	}

	// Symlink host Claude config
	symlinks := [][2]string{
		{opts.ClaudeDir + "/.claude.json", "/home/agent/.claude.json"},
		{opts.ClaudeDir + "/settings.json", "/home/agent/.claude/settings.json"},
		{opts.ClaudeDir + "/plugins", "/home/agent/.claude/plugins"},
	}
	for _, sl := range symlinks {
		if _, err := m.docker.SandboxExec(sandboxName, "ln", "-sf", sl[0], sl[1]); err != nil {
			return fmt.Errorf("symlinking %s: %w", sl[1], err)
		}
	}

	// Copy workspace, clean, and create branch
	script := fmt.Sprintf(
		`cp -a '%s/.' /home/agent/workspace/ && cd /home/agent/workspace && git clean -fdx -q && git checkout -b '%s'`,
		opts.Workspace, opts.SessionID)
	if _, err := m.docker.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
		return fmt.Errorf("copying workspace: %w", err)
	}
	return nil
}

// ApplyNetworkPolicy reads allowed-hosts.txt and applies deny-by-default network policy.
// Returns true if a policy was applied.
func (m *Manager) ApplyNetworkPolicy(sandboxName, template string) (bool, error) {
	hostsFile := filepath.Join(m.templatesDir, template, "allowed-hosts.txt")
	f, err := os.Open(hostsFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()

	var hosts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		hosts = append(hosts, line)
	}
	if err := m.docker.SandboxNetworkProxy(sandboxName, hosts); err != nil {
		return false, fmt.Errorf("applying network policy: %w", err)
	}
	return true, nil
}

// VerifyNetworkPolicy checks that the firewall blocks example.com and allows api.github.com.
func (m *Manager) VerifyNetworkPolicy(sandboxName string) error {
	_, err := m.docker.SandboxExec(sandboxName,
		"curl", "--connect-timeout", "5", "-sf", "https://example.com")
	if err == nil {
		return fmt.Errorf("firewall verification failed - was able to reach https://example.com")
	}
	_, err = m.docker.SandboxExec(sandboxName,
		"curl", "--connect-timeout", "5", "-sf", "https://api.github.com/zen")
	if err != nil {
		return fmt.Errorf("firewall verification failed - unable to reach https://api.github.com")
	}
	return nil
}

// WrapClaudeBinary wraps the claude binary to cd to /home/agent/workspace first.
func (m *Manager) WrapClaudeBinary(sandboxName string) error {
	script := `CLAUDE_BIN=$(which claude)
if [ ! -f "${CLAUDE_BIN}-real" ]; then
  sudo mv "$CLAUDE_BIN" "${CLAUDE_BIN}-real"
fi
sudo tee "$CLAUDE_BIN" > /dev/null << 'WRAPPER'
#!/bin/bash
cd /home/agent/workspace
exec "$(dirname "$0")/claude-real" "$@"
WRAPPER
sudo chmod +x "$CLAUDE_BIN"`
	if _, err := m.docker.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
		return fmt.Errorf("wrapping claude binary: %w", err)
	}
	return nil
}

// Run starts a sandbox.
func (m *Manager) Run(sandboxName string, args ...string) error {
	return m.docker.SandboxRun(sandboxName, args...)
}

// List returns sandbox names matching the prefix.
func (m *Manager) List(workspacePrefix string) ([]string, error) {
	sandboxes, err := m.docker.SandboxLs(workspacePrefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, s := range sandboxes {
		names = append(names, s.Name)
	}
	return names, nil
}

// Remove deletes a single sandbox.
func (m *Manager) Remove(name string) error {
	return m.docker.SandboxRm(name)
}

// RemoveAll deletes all sandboxes matching the prefix. Returns count removed.
func (m *Manager) RemoveAll(workspacePrefix string) (int, error) {
	sandboxes, err := m.docker.SandboxLs(workspacePrefix)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, s := range sandboxes {
		if err := m.docker.SandboxRm(s.Name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", s.Name, err)
			continue
		}
		count++
	}
	return count, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/sandbox/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/sandbox/
git commit -m "feat: add sandbox manager with lifecycle operations"
```

---

### Task 5: Credentials Package (TDD)

**Files:**
- Create: `internal/credentials/keychain.go`
- Create: `internal/credentials/keychain_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/credentials/keychain_test.go
package credentials

import (
	"claudebox/internal/docker"
	"fmt"
	"strings"
	"testing"
)

type mockDocker struct {
	execCalls [][]string
}

func (m *mockDocker) Build(string, string) error                            { return nil }
func (m *mockDocker) SandboxCreate(string, docker.SandboxCreateOpts) error  { return nil }
func (m *mockDocker) SandboxRun(string, ...string) error                    { return nil }
func (m *mockDocker) SandboxExec(name string, args ...string) (string, error) {
	m.execCalls = append(m.execCalls, append([]string{name}, args...))
	return "", nil
}
func (m *mockDocker) SandboxLs(string) ([]docker.SandboxInfo, error) { return nil, nil }
func (m *mockDocker) SandboxRm(string) error                         { return nil }
func (m *mockDocker) SandboxNetworkProxy(string, []string) error     { return nil }

func TestRefreshWithCredentials(t *testing.T) {
	md := &mockDocker{}
	orig := readKeychainFn
	readKeychainFn = func() (string, error) { return `{"token":"abc123"}`, nil }
	defer func() { readKeychainFn = orig }()

	if err := Refresh(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if len(md.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(md.execCalls))
	}
	joined := strings.Join(md.execCalls[0], " ")
	if !strings.Contains(joined, "base64 -d") || !strings.Contains(joined, ".credentials.json") {
		t.Errorf("exec should decode base64 to .credentials.json: got %s", joined)
	}
}

func TestRefreshWithNoCredentials(t *testing.T) {
	md := &mockDocker{}
	orig := readKeychainFn
	readKeychainFn = func() (string, error) { return "", fmt.Errorf("not found") }
	defer func() { readKeychainFn = orig }()

	if err := Refresh(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if len(md.execCalls) != 0 {
		t.Errorf("expected 0 exec calls with no creds, got %d", len(md.execCalls))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/credentials/
```

- [ ] **Step 3: Implement keychain.go**

```go
// internal/credentials/keychain.go
package credentials

import (
	"claudebox/internal/docker"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readKeychainFn reads credentials from macOS Keychain. Replaceable for testing.
var readKeychainFn = readKeychain

func readKeychain() (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Refresh reads credentials from macOS Keychain and injects them into the sandbox.
func Refresh(d docker.Docker, sandboxName string) error {
	creds, err := readKeychainFn()
	if err != nil || creds == "" {
		if err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: No credentials found in Keychain. You may need to re-authenticate inside the sandbox.")
		}
		return nil
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(creds))
	script := fmt.Sprintf(
		"echo '%s' | tr -d '[:space:]' | base64 -d > /home/agent/.claude/.credentials.json && chmod 600 /home/agent/.claude/.credentials.json",
		encoded)
	_, err = d.SandboxExec(sandboxName, "sh", "-c", script)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/credentials/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/credentials/
git commit -m "feat: add credentials package for Keychain integration"
```

---

### Task 6: Environment Package (TDD)

**Files:**
- Create: `internal/environment/environment.go`
- Create: `internal/environment/environment_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/environment/environment_test.go
package environment

import (
	"claudebox/internal/docker"
	"strings"
	"testing"
)

type mockDocker struct {
	execCalls [][]string
}

func (m *mockDocker) Build(string, string) error                            { return nil }
func (m *mockDocker) SandboxCreate(string, docker.SandboxCreateOpts) error  { return nil }
func (m *mockDocker) SandboxRun(string, ...string) error                    { return nil }
func (m *mockDocker) SandboxExec(name string, args ...string) (string, error) {
	m.execCalls = append(m.execCalls, append([]string{name}, args...))
	return "", nil
}
func (m *mockDocker) SandboxLs(string) ([]docker.SandboxInfo, error) { return nil, nil }
func (m *mockDocker) SandboxRm(string) error                         { return nil }
func (m *mockDocker) SandboxNetworkProxy(string, []string) error     { return nil }

func TestSetupTruncatesPersistentEnv(t *testing.T) {
	md := &mockDocker{}
	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if len(md.execCalls) < 1 {
		t.Fatal("expected at least 1 exec call")
	}
	first := strings.Join(md.execCalls[0], " ")
	if !strings.Contains(first, "truncate") {
		t.Errorf("first call should truncate persistent env: got %s", first)
	}
}

func TestSetupExportsGitHubUsername(t *testing.T) {
	md := &mockDocker{}
	t.Setenv("GITHUB_USERNAME", "testuser")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range md.execCalls {
		if strings.Contains(strings.Join(c, " "), "GITHUB_USERNAME") {
			found = true
		}
	}
	if !found {
		t.Error("should export GITHUB_USERNAME when set")
	}
}

func TestSetupConfiguresJVMProxy(t *testing.T) {
	md := &mockDocker{}
	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range md.execCalls {
		joined := strings.Join(c, " ")
		if strings.Contains(joined, "HTTPS_PROXY") || strings.Contains(joined, "keytool") {
			found = true
		}
	}
	if !found {
		t.Error("should configure JVM proxy settings")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/environment/
```

- [ ] **Step 3: Implement environment.go**

```go
// internal/environment/environment.go
package environment

import (
	"claudebox/internal/docker"
	"fmt"
	"os"
)

// Setup configures the sandbox environment: truncates persistent env,
// exports GITHUB_USERNAME, configures JVM proxy, imports CA cert.
func Setup(d docker.Docker, sandboxName string) error {
	// Truncate persistent env to avoid duplicates on resume
	if _, err := d.SandboxExec(sandboxName, "sh", "-c",
		"sudo truncate -s 0 /etc/sandbox-persistent.sh"); err != nil {
		return fmt.Errorf("truncating persistent env: %w", err)
	}

	// Export GITHUB_USERNAME if set
	if username := os.Getenv("GITHUB_USERNAME"); username != "" {
		script := fmt.Sprintf("echo 'export GITHUB_USERNAME=%s' >> /etc/sandbox-persistent.sh", username)
		if _, err := d.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
			return fmt.Errorf("setting GITHUB_USERNAME: %w", err)
		}
	}

	// Configure JVM proxy and import CA cert
	jvmScript := `if [ -n "$HTTPS_PROXY" ]; then
  PROXY_HOST=$(echo "$HTTPS_PROXY" | sed -E "s|https?://||;s|:.*||")
  PROXY_PORT=$(echo "$HTTPS_PROXY" | sed -E "s|.*:([0-9]+).*|\1|")
  echo "export JAVA_TOOL_OPTIONS=\"-Dhttp.proxyHost=${PROXY_HOST} -Dhttp.proxyPort=${PROXY_PORT} -Dhttps.proxyHost=${PROXY_HOST} -Dhttps.proxyPort=${PROXY_PORT}\"" >> /etc/sandbox-persistent.sh
fi
JAVA_HOME=$(java -XshowSettings:properties 2>&1 | grep "java.home" | awk "{print \$3}")
PROXY_CERT=$(find /usr/local/share/ca-certificates -name "*.crt" 2>/dev/null | head -1)
if [ -n "$PROXY_CERT" ] && [ -n "$JAVA_HOME" ]; then
  sudo keytool -import -trustcacerts -cacerts -storepass changeit -noprompt -alias proxy-ca -file "$PROXY_CERT" 2>/dev/null || true
fi`
	if _, err := d.SandboxExec(sandboxName, "sh", "-c", jvmScript); err != nil {
		return fmt.Errorf("configuring JVM proxy: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/environment/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/environment/
git commit -m "feat: add environment package for sandbox env setup"
```

---

### Task 7: ls and rm Commands (TDD)

**Files:**
- Create: `internal/commands/ls.go`
- Create: `internal/commands/rm.go`
- Create: `internal/commands/commands_test.go`

- [ ] **Step 1: Write shared test mock and command tests**

```go
// internal/commands/commands_test.go
package commands

import (
	"bytes"
	"claudebox/internal/docker"
	"fmt"
	"testing"
)

type mockDocker struct {
	lsOutput []docker.SandboxInfo
	rmCalls  []string
	failRm   bool
}

func (m *mockDocker) Build(string, string) error                            { return nil }
func (m *mockDocker) SandboxCreate(string, docker.SandboxCreateOpts) error  { return nil }
func (m *mockDocker) SandboxRun(string, ...string) error                    { return nil }
func (m *mockDocker) SandboxExec(string, ...string) (string, error)         { return "", nil }
func (m *mockDocker) SandboxLs(filter string) ([]docker.SandboxInfo, error) {
	if filter == "" {
		return m.lsOutput, nil
	}
	var out []docker.SandboxInfo
	for _, s := range m.lsOutput {
		if len(s.Name) >= len(filter) && s.Name[:len(filter)] == filter {
			out = append(out, s)
		}
	}
	return out, nil
}
func (m *mockDocker) SandboxRm(name string) error {
	m.rmCalls = append(m.rmCalls, name)
	if m.failRm {
		return fmt.Errorf("rm failed")
	}
	return nil
}
func (m *mockDocker) SandboxNetworkProxy(string, []string) error { return nil }

func TestLsCommand(t *testing.T) {
	md := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: "sandbox-1"}, {Name: "sandbox-2"},
	}}
	cmd := NewLsCmd(md)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "sandbox-1\nsandbox-2\n" {
		t.Errorf("ls output: got %q", out.String())
	}
}

func TestRmByName(t *testing.T) {
	md := &mockDocker{lsOutput: []docker.SandboxInfo{{Name: "my-sandbox"}}}
	cmd := NewRmCmd(md)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"my-sandbox"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(md.rmCalls) != 1 || md.rmCalls[0] != "my-sandbox" {
		t.Errorf("rm calls: got %v", md.rmCalls)
	}
}

func TestRmNotFound(t *testing.T) {
	md := &mockDocker{lsOutput: []docker.SandboxInfo{}}
	cmd := NewRmCmd(md)
	cmd.SetArgs([]string{"nonexistent"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Error("rm should fail when sandbox not found")
	}
}

func TestRmNoArgs(t *testing.T) {
	md := &mockDocker{}
	cmd := NewRmCmd(md)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err == nil {
		t.Error("rm with no args should fail")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/commands/
```

- [ ] **Step 3: Implement ls.go**

```go
// internal/commands/ls.go
package commands

import (
	"claudebox/internal/docker"
	"fmt"

	"github.com/spf13/cobra"
)

// NewLsCmd returns the cobra command for claudebox ls.
func NewLsCmd(d docker.Docker) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all sandboxes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sandboxes, err := d.SandboxLs("")
			if err != nil {
				return err
			}
			for _, s := range sandboxes {
				fmt.Fprintln(cmd.OutOrStdout(), s.Name)
			}
			return nil
		},
	}
}
```

- [ ] **Step 4: Implement rm.go**

```go
// internal/commands/rm.go
package commands

import (
	"claudebox/internal/docker"
	"claudebox/internal/sandbox"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewRmCmd returns the cobra command for claudebox rm.
func NewRmCmd(d docker.Docker) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name|all>",
		Short: "Remove a sandbox or all sandboxes for the current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := sandbox.NewManager(d, "")

			if args[0] == "all" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				prefix := sandbox.SanitizeWorkspaceName(filepath.Base(wd)) + "-"
				count, err := mgr.RemoveAll(prefix)
				if err != nil {
					return err
				}
				if count == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No sandboxes found for %s.\n", wd)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Removed %d sandbox(es).\n", count)
				}
				return nil
			}

			// Remove by name
			name := args[0]
			all, err := d.SandboxLs("")
			if err != nil {
				return err
			}
			found := false
			for _, s := range all {
				if s.Name == name {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("sandbox %s not found", name)
			}
			if err := mgr.Remove(name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed sandbox: %s\n", name)
			return nil
		},
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/commands/ -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/commands/
git commit -m "feat: add ls and rm commands"
```

---

### Task 8: create Command (TDD)

**Files:**
- Create: `internal/commands/create.go`
- Modify: `internal/commands/commands_test.go`

- [ ] **Step 1: Add create tests to commands_test.go**

```go
// Add to internal/commands/commands_test.go

func TestParseCreateArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		template   string
		workspace  string // empty means default to cwd
		agentArgs  []string
	}{
		{"template only", []string{"jvm"}, "jvm", "", nil},
		{"template and workspace", []string{"jvm", "/path"}, "jvm", "/path", nil},
		{"template with agent args", []string{"jvm", "--dangerously-skip-permissions"}, "jvm", "", []string{"--dangerously-skip-permissions"}},
		{"all three", []string{"jvm", "/path", "--skip"}, "jvm", "/path", []string{"--skip"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, ws, agentArgs := ParseCreateArgs(tt.args)
			if tmpl != tt.template {
				t.Errorf("template: got %q, want %q", tmpl, tt.template)
			}
			if tt.workspace != "" && ws != tt.workspace {
				t.Errorf("workspace: got %q, want %q", ws, tt.workspace)
			}
			if len(agentArgs) != len(tt.agentArgs) {
				t.Errorf("agentArgs: got %v, want %v", agentArgs, tt.agentArgs)
			}
		})
	}
}

func TestRunCreateValidatesTemplate(t *testing.T) {
	md := &mockDocker{}
	// Empty templates dir — no templates exist
	err := RunCreate(md, t.TempDir(), []string{"nonexistent"})
	if err == nil {
		t.Error("should fail with invalid template")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/commands/
```

- [ ] **Step 3: Implement create.go**

```go
// internal/commands/create.go
package commands

import (
	"claudebox/internal/credentials"
	"claudebox/internal/docker"
	"claudebox/internal/environment"
	"claudebox/internal/sandbox"
	"fmt"
	"os"
	"strings"
)

// ParseCreateArgs parses [template] [workspace] [agent_args...] from positional args.
// Cobra consumes the "--" separator, so agent args appear as regular positional args.
// Convention: workspace doesn't start with "-", agent args do.
func ParseCreateArgs(args []string) (template, workspace string, agentArgs []string) {
	if len(args) == 0 {
		return
	}
	template = args[0]
	rest := args[1:]
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		workspace = rest[0]
		rest = rest[1:]
	}
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	agentArgs = rest
	return
}

// RunCreate executes the create flow: validate, build, create sandbox, setup, run.
func RunCreate(d docker.Docker, templatesDir string, args []string) error {
	template, workspace, agentArgs := ParseCreateArgs(args)
	if template == "" {
		return fmt.Errorf("template name required")
	}

	mgr := sandbox.NewManager(d, templatesDir)

	// 1. Validate template
	if err := mgr.ValidateTemplate(template); err != nil {
		return err
	}

	// 2. Build image
	fmt.Printf("Building template image: %s-sandbox...\n", template)
	imageName, err := mgr.BuildImage(template)
	if err != nil {
		return err
	}

	// 3. Generate names
	sessionID := sandbox.GenerateSessionID()
	sandboxName := sandbox.GenerateSandboxName(workspace, template)
	claudeDir := os.Getenv("HOME") + "/.claude"

	// 4. Create sandbox
	fmt.Printf("Creating sandbox: %s...\n", sandboxName)
	if err := mgr.Create(sandboxName, sandbox.CreateOpts{
		ImageName: imageName,
		Workspace: workspace,
		ClaudeDir: claudeDir,
		SessionID: sessionID,
	}); err != nil {
		return err
	}

	// 5. Setup environment
	if err := environment.Setup(d, sandboxName); err != nil {
		return err
	}

	// 6. Apply and verify network policy
	fmt.Println("Applying network policy (deny by default)...")
	applied, err := mgr.ApplyNetworkPolicy(sandboxName, template)
	if err != nil {
		return err
	}
	if applied {
		fmt.Println("Verifying network policy...")
		if err := mgr.VerifyNetworkPolicy(sandboxName); err != nil {
			return err
		}
		fmt.Println("Network policy verified.")
	} else {
		fmt.Println("No allowed-hosts.txt found, using default network policy (allow all).")
	}

	// 7. Refresh credentials
	if err := credentials.Refresh(d, sandboxName); err != nil {
		return err
	}

	// 8. Wrap claude binary
	if err := mgr.WrapClaudeBinary(sandboxName); err != nil {
		return err
	}

	// 9. Run
	fmt.Println("Starting sandbox...")
	runArgs := append([]string{"--dangerously-skip-permissions"}, agentArgs...)
	return mgr.Run(sandboxName, runArgs...)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/commands/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/commands/create.go internal/commands/commands_test.go
git commit -m "feat: add create command"
```

---

### Task 9: resume Command (TDD)

**Files:**
- Create: `internal/commands/resume.go`
- Modify: `internal/commands/commands_test.go`

- [ ] **Step 1: Add resume tests**

```go
// Add to internal/commands/commands_test.go

func TestResumeNoSandboxes(t *testing.T) {
	md := &mockDocker{lsOutput: []docker.SandboxInfo{}}
	cmd := NewResumeCmd(md, t.TempDir())
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("resume should fail with no sandboxes")
	}
}
```

The interactive picker (stdin prompts) is hard to unit test without complex mocking. Test coverage for that path comes from integration tests and manual testing. The unit test verifies the error-on-no-sandboxes path.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/commands/
```

- [ ] **Step 3: Implement resume.go**

```go
// internal/commands/resume.go
package commands

import (
	"bufio"
	"claudebox/internal/credentials"
	"claudebox/internal/docker"
	"claudebox/internal/environment"
	"claudebox/internal/sandbox"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// NewResumeCmd returns the cobra command for claudebox resume.
func NewResumeCmd(d docker.Docker, templatesDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "resume [-- agent_args...]",
		Short: "Resume an existing sandbox",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResume(d, templatesDir, args, os.Stdin)
		},
	}
}

func runResume(d docker.Docker, templatesDir string, agentArgs []string, stdin *os.File) error {
	mgr := sandbox.NewManager(d, templatesDir)
	reader := bufio.NewReader(stdin)

	// List sandboxes for current workspace
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	prefix := sandbox.SanitizeWorkspaceName(filepath.Base(wd)) + "-"
	names, err := mgr.List(prefix)
	if err != nil {
		return err
	}

	if len(names) == 0 {
		return fmt.Errorf("no sandboxes found for this workspace")
	}

	var sandboxName string

	if len(names) == 1 {
		fmt.Printf("Resume %s? [Y/n]: ", names[0])
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "n") {
			return nil
		}
		sandboxName = names[0]
	} else {
		fmt.Println("Available sandboxes:")
		for i, name := range names {
			fmt.Printf("  %d) %s\n", i+1, name)
		}
		for {
			fmt.Printf("Pick a sandbox [1-%d]: ", len(names))
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			pick, err := strconv.Atoi(line)
			if err == nil && pick >= 1 && pick <= len(names) {
				sandboxName = names[pick-1]
				break
			}
			fmt.Printf("Invalid selection. Enter a number between 1 and %d.\n", len(names))
		}
	}

	fmt.Printf("Resuming sandbox: %s...\n", sandboxName)

	// Environment first, then credentials (matches Bash ordering)
	if err := environment.Setup(d, sandboxName); err != nil {
		return err
	}
	if err := credentials.Refresh(d, sandboxName); err != nil {
		return err
	}
	if err := mgr.WrapClaudeBinary(sandboxName); err != nil {
		return err
	}

	fmt.Println("Starting sandbox...")
	runArgs := append([]string{"--dangerously-skip-permissions"}, agentArgs...)
	return mgr.Run(sandboxName, runArgs...)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/commands/ -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/commands/resume.go internal/commands/commands_test.go
git commit -m "feat: add resume command"
```

---

### Task 10: Wire Up main.go

**Files:**
- Modify: `cmd/claudebox/main.go`

- [ ] **Step 1: Replace stubs with real commands**

```go
// cmd/claudebox/main.go
package main

import (
	"claudebox/internal/commands"
	"claudebox/internal/docker"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func main() {
	templatesDir, err := findTemplatesDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	d := docker.NewClient()

	rootCmd := &cobra.Command{
		Use:   "claudebox [template] [workspace] [-- agent_args...]",
		Short: "Run Claude Code in sandboxed Docker containers",
		Long: `claudebox creates isolated Docker sandbox environments for Claude Code
with per-template toolchains and network restrictions.

Each run creates a new sandbox with a local copy of the repo,
so multiple sessions can work on independent branches in parallel.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return commands.RunCreate(d, templatesDir, args)
		},
	}

	rootCmd.AddCommand(
		commands.NewLsCmd(d),
		commands.NewRmCmd(d),
		commands.NewResumeCmd(d, templatesDir),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func findTemplatesDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("cannot resolve symlinks: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "templates"), nil
}
```

- [ ] **Step 2: Verify all unit tests pass**

```bash
go test ./...
```

- [ ] **Step 3: Build and verify help output**

```bash
go build -o cb-go ./cmd/claudebox && ./cb-go --help
```

Expected: help showing usage, with `ls`, `rm`, `resume` subcommands and the root command accepting `[template]`.

- [ ] **Step 4: Commit**

```bash
git add cmd/claudebox/main.go
git commit -m "feat: wire up main.go with all commands"
```

---

### Task 11: Integration Test Infrastructure

**Files:**
- Create: `tests/integration/helpers_test.go`

- [ ] **Step 1: Create integration test infrastructure**

```go
//go:build integration

// tests/integration/helpers_test.go
package integration

import (
	"claudebox/internal/docker"
	"claudebox/internal/environment"
	"claudebox/internal/sandbox"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var (
	testDocker     *docker.Client
	testManager    *sandbox.Manager
	templatesDir   string
	activeSandbox  string
)

func TestMain(m *testing.M) {
	// Skip if docker sandbox is not available
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Println("SKIP: docker not found")
		os.Exit(0)
	}
	cmd := exec.Command("docker", "sandbox", "ls")
	if err := cmd.Run(); err != nil {
		fmt.Println("SKIP: docker sandbox not available")
		os.Exit(0)
	}

	// Resolve templates dir relative to this test file
	_, filename, _, _ := runtime.Caller(0)
	templatesDir = filepath.Join(filepath.Dir(filename), "..", "..", "templates")

	testDocker = docker.NewClient()
	testManager = sandbox.NewManager(testDocker, templatesDir)

	os.Exit(m.Run())
}

func buildTemplateImage(t *testing.T, template string) {
	t.Helper()
	_, err := testManager.BuildImage(template)
	if err != nil {
		t.Fatalf("failed to build image: %v", err)
	}
}

func createTestSandbox(t *testing.T, template, workspace string) string {
	t.Helper()
	sessionID := sandbox.GenerateSessionID()
	name := sandbox.GenerateSandboxName(workspace, template)
	activeSandbox = name

	err := testManager.Create(name, sandbox.CreateOpts{
		ImageName: template + "-sandbox",
		Workspace: workspace,
		ClaudeDir: os.Getenv("HOME") + "/.claude",
		SessionID: sessionID,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	if err := environment.Setup(testDocker, name); err != nil {
		t.Fatalf("failed to setup environment: %v", err)
	}
	if err := testManager.WrapClaudeBinary(name); err != nil {
		t.Fatalf("failed to wrap claude binary: %v", err)
	}

	return name
}

func applyNetworkPolicy(t *testing.T, sandboxName, template string) {
	t.Helper()
	applied, err := testManager.ApplyNetworkPolicy(sandboxName, template)
	if err != nil {
		t.Fatalf("failed to apply network policy: %v", err)
	}
	if !applied {
		t.Fatal("expected network policy to be applied")
	}
}

func cleanupSandbox(t *testing.T, name string) {
	t.Helper()
	_ = testManager.Remove(name)
}

func createTestWorkspace(t *testing.T, dirname string) string {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), dirname)
	os.MkdirAll(workspace, 0o755)

	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", workspace}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s", args, out)
		}
	}
	run("init", "-q")
	os.WriteFile(filepath.Join(workspace, "testfile.txt"), []byte("test content"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "init")

	return workspace
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go test -tags integration -c ./tests/integration/ -o /dev/null
```

Expected: compiles without errors (doesn't run tests yet).

- [ ] **Step 3: Commit**

```bash
git add tests/integration/helpers_test.go
git commit -m "feat: add integration test infrastructure"
```

---

### Task 12: Integration Tests

**Files:**
- Create: `tests/integration/create_test.go`
- Create: `tests/integration/filesystem_test.go`
- Create: `tests/integration/network_test.go`

- [ ] **Step 1: Create create_test.go**

```go
//go:build integration

package integration

import (
	"regexp"
	"testing"
)

func TestImageBuilds(t *testing.T) {
	buildTemplateImage(t, "jvm")
}

func TestSandboxNameFormat(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-create-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	pattern := `^cb-create-test-jvm-sandbox-\d{8}-\d{6}$`
	if matched, _ := regexp.MatchString(pattern, name); !matched {
		t.Errorf("sandbox name %q doesn't match %s", name, pattern)
	}
}
```

- [ ] **Step 2: Create filesystem_test.go**

```go
//go:build integration

package integration

import (
	"strings"
	"testing"
)

func TestFilesystemLayout(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-fs-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	t.Run("workspace files exist", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "test", "-f", "/home/agent/workspace/testfile.txt")
		if err != nil {
			t.Error("testfile.txt should exist in workspace")
		}
	})

	t.Run("git branch matches sandbox pattern", func(t *testing.T) {
		branch, err := testDocker.SandboxExec(name, "git", "-C", "/home/agent/workspace", "branch", "--show-current")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(branch, "sandbox-") {
			t.Errorf("branch %q should start with sandbox-", branch)
		}
	})

	t.Run("claude config symlinks exist", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "test", "-L", "/home/agent/.claude.json")
		if err != nil {
			t.Error(".claude.json symlink should exist")
		}
	})

	t.Run("claude binary wrapper has cd workspace", func(t *testing.T) {
		out, err := testDocker.SandboxExec(name, "sh", "-c", `cat "$(which claude)"`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "cd /home/agent/workspace") {
			t.Errorf("claude wrapper should contain cd: got %s", out)
		}
	})
}
```

- [ ] **Step 3: Create network_test.go**

```go
//go:build integration

package integration

import (
	"os"
	"os/exec"
	"testing"
)

func TestNetworkPolicy(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-net-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	applyNetworkPolicy(t, name, "jvm")

	t.Run("blocked host is unreachable", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name,
			"curl", "--connect-timeout", "5", "-sf", "https://example.com")
		if err == nil {
			t.Error("example.com should be blocked")
		}
	})

	t.Run("allowed host is reachable", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name,
			"curl", "--connect-timeout", "10", "-sf", "https://api.github.com/zen")
		if err != nil {
			t.Error("api.github.com should be reachable")
		}
	})
}

func TestNoNetworkPolicyAllowsAll(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-nofilt-test")

	// Build image from a temp template with no allowed-hosts.txt
	tmpDir := t.TempDir()
	// Copy Dockerfile only (no allowed-hosts.txt)
	src, _ := os.ReadFile(templatesDir + "/jvm/Dockerfile")
	os.WriteFile(tmpDir+"/Dockerfile", src, 0o644)

	cmd := exec.Command("docker", "build", "-q", "-t", "nofilter-sandbox", tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s", out)
	}
	defer exec.Command("docker", "rmi", "nofilter-sandbox").Run()

	// Create minimal sandbox
	name := "cb-nofilt-test-sandbox"
	homeDir := os.Getenv("HOME")
	cmd = exec.Command("docker", "sandbox", "create", "-t", "nofilter-sandbox",
		"--name", name, "claude", workspace, homeDir+"/.claude")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create failed: %s", out)
	}
	defer func() { exec.Command("docker", "sandbox", "rm", name).Run() }()

	// Without network policy, example.com should be reachable
	_, err := testDocker.SandboxExec(name,
		"curl", "--connect-timeout", "10", "-sf", "https://example.com")
	if err != nil {
		t.Error("example.com should be reachable without network policy")
	}
}
```

- [ ] **Step 4: Run integration tests (requires Docker Desktop with sandbox support)**

```bash
go test -tags integration -v ./tests/integration/ -timeout 300s
```

Expected: all tests pass if Docker sandbox is available, or skip gracefully if not.

- [ ] **Step 5: Commit**

```bash
git add tests/integration/
git commit -m "test: add Go integration tests for create, filesystem, and network"
```

---

### Task 13: Makefile, Bash Removal, README Update

**Files:**
- Modify: `Makefile`
- Delete: `claudebox` (bash entry point)
- Delete: `src/` (entire directory)
- Delete: `tests/unit/`, `tests/test_helper/`, `tests/setup_test_deps.sh`
- Modify: `README.md`

- [ ] **Step 1: Update Makefile**

```makefile
.PHONY: build test test-unit test-integration test-all clean

build:
	go build -o claudebox ./cmd/claudebox

test: test-unit

test-unit:
	go test ./...

test-integration:
	go test -tags integration -v ./tests/integration/ -timeout 300s

test-all: test-unit test-integration

clean:
	rm -f claudebox
```

- [ ] **Step 2: Verify unit tests pass**

```bash
make test
```

- [ ] **Step 3: Remove Bash source and BATS test infrastructure**

```bash
rm claudebox
rm -rf src/
rm -rf tests/unit/ tests/test_helper/ tests/setup_test_deps.sh
```

- [ ] **Step 4: Build the Go binary**

```bash
make build
```

- [ ] **Step 5: Update README.md**

Update the installation, usage, and development sections to reflect Go:
- Build: `go build -o claudebox ./cmd/claudebox`
- Or: `make build`
- Tests: `make test` / `make test-integration` / `make test-all`
- Remove references to Bash, BATS, `setup_test_deps.sh`

- [ ] **Step 6: Run integration tests with the Go binary**

```bash
make test-integration
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: complete Go migration, remove Bash source"
```
