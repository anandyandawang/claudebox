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
