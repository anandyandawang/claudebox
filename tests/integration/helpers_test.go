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

func createTestSandbox(t *testing.T, template, workspace string) string {
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
