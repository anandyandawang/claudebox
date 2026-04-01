//go:build integration

package integration

import (
	"claudebox/internal/sandbox"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFilesystemLayout(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-fs-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	t.Run("workspace files exist", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "test", "-f", sandbox.SandboxWorkspace+"/testfile.txt")
		if err != nil {
			t.Error("testfile.txt should exist in workspace")
		}
	})

	t.Run("git branch matches sandbox pattern", func(t *testing.T) {
		branch, err := testDocker.SandboxExec(name, "git", "-C", sandbox.SandboxWorkspace, "branch", "--show-current")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(branch, "sandbox-") {
			t.Errorf("branch %q should start with sandbox-", branch)
		}
	})

	t.Run("claude config symlinks exist", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "test", "-L", sandbox.SandboxHome+"/.claude.json")
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

	t.Run("plugin paths rewritten to sandbox paths", func(t *testing.T) {
		manifest := sandbox.SandboxClaudeDir + "/plugins/installed_plugins.json"
		out, err := testDocker.SandboxExec(name, "cat", manifest)
		if err != nil {
			t.Skipf("no installed_plugins.json: %v", err)
		}
		if strings.Contains(out, os.Getenv("HOME")) {
			t.Errorf("installed_plugins.json still contains host path %s", os.Getenv("HOME"))
		}
		if !strings.Contains(out, sandbox.SandboxClaudeDir) {
			t.Errorf("installed_plugins.json should contain sandbox path %s", sandbox.SandboxClaudeDir)
		}
	})

	t.Run("sandbox run starts without cwd error", func(t *testing.T) {
		// Verifies the OCI runtime can chdir into the workspace mount.
		// This catches the deleted-temp-dir bug where docker sandbox run
		// fails with "no such file or directory" before any command runs.
		cmd := exec.Command("docker", "sandbox", "run", name, "--", "--version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("sandbox run failed (cwd likely invalid): %s", out)
		}
	})
}
