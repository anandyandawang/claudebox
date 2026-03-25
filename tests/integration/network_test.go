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

	tmpDir := t.TempDir()
	src, _ := os.ReadFile(templatesDir + "/jvm/Dockerfile")
	os.WriteFile(tmpDir+"/Dockerfile", src, 0o644)

	cmd := exec.Command("docker", "build", "-q", "-t", "nofilter-sandbox", tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s", out)
	}
	defer exec.Command("docker", "rmi", "nofilter-sandbox").Run()

	name := "cb-nofilt-test-sandbox"
	homeDir := os.Getenv("HOME")
	cmd = exec.Command("docker", "sandbox", "create", "-t", "nofilter-sandbox",
		"--name", name, "claude", workspace, homeDir+"/.claude")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create failed: %s", out)
	}
	defer func() { exec.Command("docker", "sandbox", "rm", name).Run() }()

	_, err := testDocker.SandboxExec(name,
		"curl", "--connect-timeout", "10", "-sf", "https://example.com")
	if err != nil {
		t.Error("example.com should be reachable without network policy")
	}
}
