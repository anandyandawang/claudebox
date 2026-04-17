package environment

import (
	"claudebox/internal/docker"
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
