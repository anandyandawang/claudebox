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

// withGitIdentity swaps readGitIdentityFn for the duration of a test.
func withGitIdentity(t *testing.T, name, email string) {
	t.Helper()
	orig := readGitIdentityFn
	readGitIdentityFn = func() (string, string) { return name, email }
	t.Cleanup(func() { readGitIdentityFn = orig })
}

// hasGitConfigCall returns true if execCalls contains a `git config --global <key> <value>` call.
func hasGitConfigCall(calls [][]string, key, value string) bool {
	for _, c := range calls {
		// c = [sandboxName, "git", "config", "--global", <key>, <value>]
		if len(c) >= 6 && c[1] == "git" && c[2] == "config" && c[3] == "--global" && c[4] == key && c[5] == value {
			return true
		}
	}
	return false
}

// hasAnyGitConfigCall returns true if execCalls contains any `git config --global <key> ...` call.
func hasAnyGitConfigCall(calls [][]string, key string) bool {
	for _, c := range calls {
		if len(c) >= 5 && c[1] == "git" && c[2] == "config" && c[3] == "--global" && c[4] == key {
			return true
		}
	}
	return false
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

func TestSetupImportsGitIdentityBoth(t *testing.T) {
	md := &mockDocker{}
	withGitIdentity(t, "Alice", "alice@example.com")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if !hasGitConfigCall(md.execCalls, "user.name", "Alice") {
		t.Error("should set git user.name=Alice")
	}
	if !hasGitConfigCall(md.execCalls, "user.email", "alice@example.com") {
		t.Error("should set git user.email=alice@example.com")
	}
}

func TestSetupImportsGitIdentityOnlyName(t *testing.T) {
	md := &mockDocker{}
	withGitIdentity(t, "Alice", "")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if !hasGitConfigCall(md.execCalls, "user.name", "Alice") {
		t.Error("should set git user.name=Alice")
	}
	if hasAnyGitConfigCall(md.execCalls, "user.email") {
		t.Error("should not set git user.email when host has none")
	}
}

func TestSetupImportsGitIdentityOnlyEmail(t *testing.T) {
	md := &mockDocker{}
	withGitIdentity(t, "", "alice@example.com")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if hasAnyGitConfigCall(md.execCalls, "user.name") {
		t.Error("should not set git user.name when host has none")
	}
	if !hasGitConfigCall(md.execCalls, "user.email", "alice@example.com") {
		t.Error("should set git user.email=alice@example.com")
	}
}

func TestSetupImportsGitIdentityNeither(t *testing.T) {
	md := &mockDocker{}
	withGitIdentity(t, "", "")

	if err := Setup(md, "my-sandbox"); err != nil {
		t.Fatal(err)
	}
	if hasAnyGitConfigCall(md.execCalls, "user.name") {
		t.Error("should not set git user.name when host has none")
	}
	if hasAnyGitConfigCall(md.execCalls, "user.email") {
		t.Error("should not set git user.email when host has none")
	}
}
