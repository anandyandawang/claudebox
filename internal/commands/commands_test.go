// internal/commands/commands_test.go
package commands

import (
	"bytes"
	"claudebox/internal/docker"
	"claudebox/internal/sandbox"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type mockDocker struct {
	lsOutput           []docker.SandboxInfo
	rmCalls            []string
	failRm             bool
	failExec           bool
	failExecWithStdin  bool
	failRun            bool
}

func (m *mockDocker) Build(string, string) error                            { return nil }
func (m *mockDocker) SandboxCreate(string, docker.SandboxCreateOpts) error  { return nil }
func (m *mockDocker) SandboxRun(_ string, _ ...string) error {
	if m.failRun {
		return fmt.Errorf("run failed")
	}
	return nil
}
func (m *mockDocker) SandboxExec(_ string, _ ...string) (string, error) {
	if m.failExec {
		return "", fmt.Errorf("exec failed")
	}
	return "", nil
}
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
func (m *mockDocker) SandboxExecWithStdin(r io.Reader, _ string, _ ...string) error {
	io.Copy(io.Discard, r) // drain to avoid pipe deadlock
	if m.failExecWithStdin {
		return fmt.Errorf("exec with stdin failed")
	}
	return nil
}
func (m *mockDocker) SandboxNetworkProxy(string, []string) error              { return nil }

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

func TestParseCreateArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		template  string
		workspace string // empty means default to cwd
		agentArgs []string
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
	err := RunCreate(md, t.TempDir(), []string{"nonexistent"})
	if err == nil {
		t.Error("should fail with invalid template")
	}
}

func TestRmAllRemovesMatchingSandboxes(t *testing.T) {
	// Create a temp directory with a known workspace name and chdir there.
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "myproject")
	if err := os.Mkdir(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(wsDir); err != nil {
		t.Fatal(err)
	}
	// Use resolved cwd for generating names (macOS resolves symlinks in Getwd)
	resolvedDir, err2 := os.Getwd()
	if err2 != nil {
		t.Fatal(err2)
	}

	// Generate sandbox names that match this workspace.
	nameA := sandbox.GenerateSandboxName(resolvedDir, sandbox.GenerateSandboxID("jvm"))
	nameB := sandbox.GenerateSandboxName(resolvedDir, sandbox.GenerateSandboxID("kotlin-spring"))

	md := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: nameA},
		{Name: nameB},
	}}

	cmd := NewRmCmd(md)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"all"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("rm all failed: %v", err)
	}

	// Both sandboxes should have been removed.
	sort.Strings(md.rmCalls)
	expected := []string{nameA, nameB}
	sort.Strings(expected)
	if len(md.rmCalls) != len(expected) {
		t.Fatalf("rm calls = %v, want %v", md.rmCalls, expected)
	}
	for i := range expected {
		if md.rmCalls[i] != expected[i] {
			t.Errorf("rm call[%d] = %q, want %q", i, md.rmCalls[i], expected[i])
		}
	}
}

func TestRmAllDoesNotRemoveDifferentWorkspace(t *testing.T) {
	// Two workspaces that truncate to the same 12 chars.
	wsNameA := "lambda-jpm-clearings"
	wsNameB := "lambda-jpm-clients"

	tmpDir := t.TempDir()
	wsDirA := filepath.Join(tmpDir, wsNameA)
	if err := os.Mkdir(wsDirA, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(wsDirA); err != nil {
		t.Fatal(err)
	}
	// Use resolved cwd (macOS resolves symlinks in Getwd)
	resolvedDirA, err2 := os.Getwd()
	if err2 != nil {
		t.Fatal(err2)
	}
	resolvedDirB := filepath.Join(filepath.Dir(resolvedDirA), wsNameB)

	prefixA := sandbox.WorkspacePrefix(resolvedDirA)
	prefixB := sandbox.WorkspacePrefix(resolvedDirB)

	// Sanity: the prefixes must differ (the hash disambiguates).
	if prefixA == prefixB {
		t.Fatalf("prefixes should differ: A=%q B=%q", prefixA, prefixB)
	}

	// Generate sandbox names for each workspace.
	sandboxA1 := sandbox.GenerateSandboxName(resolvedDirA, sandbox.GenerateSandboxID("jvm"))
	sandboxA2 := sandbox.GenerateSandboxName(resolvedDirA, sandbox.GenerateSandboxID("kotlin-spring"))
	sandboxB1 := sandbox.GenerateSandboxName(resolvedDirB, sandbox.GenerateSandboxID("jvm"))
	sandboxB2 := sandbox.GenerateSandboxName(resolvedDirB, sandbox.GenerateSandboxID("jvm"))

	md := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: sandboxA1},
		{Name: sandboxA2},
		{Name: sandboxB1},
		{Name: sandboxB2},
	}}

	cmd := NewRmCmd(md)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"all"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("rm all failed: %v", err)
	}

	// Only workspace A sandboxes should have been removed.
	for _, call := range md.rmCalls {
		if call == sandboxB1 || call == sandboxB2 {
			t.Errorf("rm removed sandbox from different workspace: %q", call)
		}
	}

	// Exactly the workspace-A sandboxes should be removed.
	removedSet := make(map[string]bool)
	for _, c := range md.rmCalls {
		removedSet[c] = true
	}
	if !removedSet[sandboxA1] {
		t.Errorf("expected %q to be removed", sandboxA1)
	}
	if !removedSet[sandboxA2] {
		t.Errorf("expected %q to be removed", sandboxA2)
	}
	if len(md.rmCalls) != 2 {
		t.Errorf("expected 2 rm calls, got %d: %v", len(md.rmCalls), md.rmCalls)
	}
}

func TestResumeOnlyShowsCurrentWorkspace(t *testing.T) {
	// Two workspaces with the same basename but different parents.
	tmpDir := t.TempDir()
	wsDirA := filepath.Join(tmpDir, "parent-a", "my-service")
	wsDirB := filepath.Join(tmpDir, "parent-b", "my-service")
	if err := os.MkdirAll(wsDirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsDirB, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(wsDirA); err != nil {
		t.Fatal(err)
	}
	resolvedDirA, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	resolvedDirB := filepath.Join(filepath.Dir(resolvedDirA), "..", "parent-b", "my-service")

	// Generate sandbox names for both workspaces.
	sandboxA := sandbox.GenerateSandboxName(resolvedDirA, sandbox.GenerateSandboxID("jvm"))
	sandboxB := sandbox.GenerateSandboxName(resolvedDirB, sandbox.GenerateSandboxID("jvm"))

	md := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: sandboxA},
		{Name: sandboxB},
	}}

	// Create a temp file with "y\n" for stdin (auto-confirm the single match).
	stdinFile, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stdinFile.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := stdinFile.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	// runResume filters by WorkspacePrefix(cwd) — should only see sandboxA.
	err = runResume(md, t.TempDir(), nil, stdinFile)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}

	// The mock's SandboxLs filters by prefix, so only sandboxA should match.
	// runResume then calls SandboxRun on the matched sandbox.
	// Verify by checking that sandboxB was never interacted with.
	// Since there's exactly 1 match, resume auto-prompts and runs it.
	// We can't directly inspect which sandbox was run (SandboxRun is a no-op mock),
	// but we can verify the prefix filtering worked by checking the ls filter.

	// The real assertion: resume didn't error, which means it found exactly 1
	// sandbox matching the current workspace prefix. If both matched, it would
	// have shown the picker (which needs "1\n" or "2\n", not "y\n") and errored.
}

func TestResumeIsolatesTruncatedWorkspaces(t *testing.T) {
	// Two workspaces that truncate to the same 12 chars ("lambda-jpm-c").
	// Resume from one must not see the other's sandboxes.
	tmpDir := t.TempDir()
	wsDirA := filepath.Join(tmpDir, "lambda-jpm-clearings")
	wsDirB := filepath.Join(tmpDir, "lambda-jpm-clients")
	if err := os.Mkdir(wsDirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(wsDirB, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(wsDirA); err != nil {
		t.Fatal(err)
	}
	resolvedDirA, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	resolvedDirB := filepath.Join(filepath.Dir(resolvedDirA), "lambda-jpm-clients")

	// Generate sandbox names for both workspaces.
	sandboxA := sandbox.GenerateSandboxName(resolvedDirA, sandbox.GenerateSandboxID("jvm"))
	sandboxB := sandbox.GenerateSandboxName(resolvedDirB, sandbox.GenerateSandboxID("jvm"))

	md := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: sandboxA},
		{Name: sandboxB},
	}}

	// Provide "y\n" — expects the single-match Y/n prompt.
	stdinFile, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stdinFile.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := stdinFile.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	// Resume from workspace A — should only see sandboxA (1 match → Y/n prompt).
	// If both matched, we'd get the picker expecting "1\n"/"2\n" and this would error.
	err = runResume(md, t.TempDir(), nil, stdinFile)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
}

func TestRmAllWithDegenerateWorkspace(t *testing.T) {
	// Workspace with a degenerate name that falls back to hash.
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "...")
	if err := os.Mkdir(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(wsDir); err != nil {
		t.Fatal(err)
	}
	resolvedDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	nameA := sandbox.GenerateSandboxName(resolvedDir, sandbox.GenerateSandboxID("jvm"))
	nameB := sandbox.GenerateSandboxName(resolvedDir, sandbox.GenerateSandboxID("jvm"))

	// Also add a sandbox from a normal workspace to verify it's not touched.
	normalDir := filepath.Join(filepath.Dir(resolvedDir), "normal-project")
	normalName := sandbox.GenerateSandboxName(normalDir, sandbox.GenerateSandboxID("jvm"))

	md := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: nameA},
		{Name: nameB},
		{Name: normalName},
	}}

	cmd := NewRmCmd(md)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"all"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("rm all failed: %v", err)
	}

	// Only the degenerate workspace's sandboxes should be removed.
	for _, call := range md.rmCalls {
		if call == normalName {
			t.Errorf("rm removed sandbox from different workspace: %q", call)
		}
	}
	if len(md.rmCalls) != 2 {
		t.Errorf("expected 2 rm calls, got %d: %v", len(md.rmCalls), md.rmCalls)
	}
}

func TestResumeWithDegenerateWorkspace(t *testing.T) {
	// Workspace with a degenerate name that falls back to hash.
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "---")
	if err := os.Mkdir(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(wsDir); err != nil {
		t.Fatal(err)
	}
	resolvedDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// One sandbox from this degenerate workspace, one from a normal workspace.
	degenerateName := sandbox.GenerateSandboxName(resolvedDir, sandbox.GenerateSandboxID("jvm"))
	normalDir := filepath.Join(filepath.Dir(resolvedDir), "normal-project")
	normalName := sandbox.GenerateSandboxName(normalDir, sandbox.GenerateSandboxID("jvm"))

	md := &mockDocker{lsOutput: []docker.SandboxInfo{
		{Name: degenerateName},
		{Name: normalName},
	}}

	stdinFile, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stdinFile.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := stdinFile.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	// Should find exactly 1 match (the degenerate one), triggering Y/n prompt.
	err = runResume(md, t.TempDir(), nil, stdinFile)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
}

func makeStdinFile(t *testing.T, content string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Seek(0, 0)
	return f
}

func setupResumeTest(t *testing.T, md *mockDocker) {
	t.Helper()
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "myproject")
	os.Mkdir(wsDir, 0o755)
	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(wsDir)
	resolved, _ := os.Getwd()
	name := sandbox.GenerateSandboxName(resolved, sandbox.GenerateSandboxID("jvm"))
	md.lsOutput = []docker.SandboxInfo{{Name: name}}
}

func TestResumeRefreshConfigFailure(t *testing.T) {
	// RefreshConfig calls SandboxExec (mkdir) then SandboxExecWithStdin (tar-pipe).
	// If SandboxExec fails, RefreshConfig returns an error.
	// Set up a fake HOME with a settings.json so collectConfigFiles finds files
	// and RefreshConfig actually reaches the exec call (not the early-return).
	tmpHome := t.TempDir()
	claudeDir := filepath.Join(tmpHome, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0o644)
	t.Setenv("HOME", tmpHome)

	md := &mockDocker{failExec: true}
	setupResumeTest(t, md)

	err := runResume(md, t.TempDir(), nil, makeStdinFile(t, "y\n"))
	if err == nil {
		t.Error("resume should fail when RefreshConfig fails")
	}
}

func TestResumeWrapBinaryFailure(t *testing.T) {
	// WrapClaudeBinary calls SandboxExec. With no config files in HOME,
	// RefreshConfig returns early. On macOS with valid Keychain credentials,
	// credentials.Refresh also reaches SandboxExec — in that case this test
	// exercises credentials.Refresh failure rather than WrapClaudeBinary, but
	// still validates that resume propagates SandboxExec errors.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	md := &mockDocker{failExec: true}
	setupResumeTest(t, md)

	err := runResume(md, t.TempDir(), nil, makeStdinFile(t, "y\n"))
	if err == nil {
		t.Error("resume should fail when WrapClaudeBinary fails")
	}
}

func TestResumeRunFailure(t *testing.T) {
	md := &mockDocker{failRun: true}
	setupResumeTest(t, md)

	err := runResume(md, t.TempDir(), nil, makeStdinFile(t, "y\n"))
	if err == nil {
		t.Error("resume should fail when SandboxRun fails")
	}
}
