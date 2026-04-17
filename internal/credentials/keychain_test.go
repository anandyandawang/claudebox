package credentials

import (
	"claudebox/internal/docker"
	"fmt"
	"io"
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
func (m *mockDocker) SandboxExecEnv(name string, _ []string, args ...string) (string, error) {
	m.execCalls = append(m.execCalls, append([]string{name}, args...))
	return "", nil
}
func (m *mockDocker) SandboxLs(string) ([]docker.SandboxInfo, error) { return nil, nil }
func (m *mockDocker) SandboxRm(string) error                         { return nil }
func (m *mockDocker) SandboxExecWithStdin(io.Reader, string, ...string) error { return nil }
func (m *mockDocker) SandboxNetworkProxy(string, []string) error              { return nil }

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
