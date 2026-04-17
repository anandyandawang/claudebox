//go:build integration

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
	testDocker   *docker.Client
	testManager  *sandbox.Manager
	templatesDir string
)

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Println("SKIP: docker not found")
		os.Exit(0)
	}
	cmd := exec.Command("docker", "sandbox", "ls")
	if err := cmd.Run(); err != nil {
		fmt.Println("SKIP: docker sandbox not available")
		os.Exit(0)
	}

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

// testSandbox holds the container name and sandbox ID for a test sandbox.
type testSandbox struct {
	name      string // container name
	sandboxID string // branch name / instance ID
}

func createTestSandbox(t *testing.T, template, workspace string) testSandbox {
	t.Helper()
	sandboxID := sandbox.GenerateSandboxID(template)
	name := sandbox.GenerateSandboxName(workspace, sandboxID)

	err := testManager.Create(name, sandbox.CreateOpts{
		ImageName: template + "-sandbox",
		Workspace: workspace,
		ClaudeDir: os.Getenv("HOME") + "/.claude",
		SessionID: sandboxID,
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

	return testSandbox{name: name, sandboxID: sandboxID}
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
	run("init", "-q", "--initial-branch=main")
	os.WriteFile(filepath.Join(workspace, "testfile.txt"), []byte("test content"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "init")

	return workspace
}

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
