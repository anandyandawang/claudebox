// internal/sandbox/sandbox_test.go
package sandbox

import (
	"claudebox/internal/docker"
	"fmt"
	"io"
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
	execOut  map[string]string
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
	m.record("SandboxCreate", name, opts.Image, opts.Command, opts.Workspace)
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

func (m *mockDocker) SandboxExecWithStdin(r io.Reader, name string, args ...string) error {
	m.record("SandboxExecWithStdin", append([]string{name}, args...)...)
	io.Copy(io.Discard, r) // drain to avoid pipe deadlock
	if m.failOn == "SandboxExecWithStdin" {
		return fmt.Errorf("exec with stdin failed")
	}
	return nil
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

	// Create real workspace dir for tar to work
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main"), 0o644)

	// Create real claude dir with config files
	claudeDir := t.TempDir()
	os.WriteFile(filepath.Join(claudeDir, ".claude.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0o644)

	err := mgr.Create("test-sandbox", CreateOpts{
		ImageName: "jvm-sandbox",
		Workspace: workspace,
		ClaudeDir: claudeDir,
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
	if createWorkspace == workspace || createWorkspace == claudeDir {
		t.Errorf("SandboxCreate should use a temp dir, not %q", createWorkspace)
	}
	// Temp dir should have been deleted (dead mount property)
	if _, err := os.Stat(createWorkspace); !os.IsNotExist(err) {
		t.Errorf("temp dir %q should have been deleted after Create", createWorkspace)
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

	// Should have SandboxExec calls for git clean and git checkout
	var hasGitClean, hasGitCheckout bool
	for _, c := range m.calls {
		if c.method != "SandboxExec" {
			continue
		}
		args := strings.Join(c.args, " ")
		if strings.Contains(args, "git") && strings.Contains(args, "clean") {
			hasGitClean = true
		}
		if strings.Contains(args, "git") && strings.Contains(args, "checkout") {
			hasGitCheckout = true
		}
	}
	if !hasGitClean {
		t.Error("expected SandboxExec call with git clean")
	}
	if !hasGitCheckout {
		t.Error("expected SandboxExec call with git checkout")
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
