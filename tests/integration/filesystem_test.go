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
	sb := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, sb.name)

	t.Run("workspace files exist", func(t *testing.T) {
		_, err := testDocker.SandboxExec(sb.name, "test", "-f", sandbox.SandboxWorkspace+"/testfile.txt")
		if err != nil {
			t.Error("testfile.txt should exist in workspace")
		}
	})

	t.Run("git branch matches sandbox ID", func(t *testing.T) {
		branch, err := testDocker.SandboxExec(sb.name, "git", "-C", sandbox.SandboxWorkspace, "branch", "--show-current")
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(branch) != sb.sandboxID {
			t.Errorf("branch = %q, want sandbox ID %q", strings.TrimSpace(branch), sb.sandboxID)
		}
	})

	t.Run("claude config symlinks exist", func(t *testing.T) {
		_, err := testDocker.SandboxExec(sb.name, "test", "-L", sandbox.SandboxHome+"/.claude.json")
		if err != nil {
			t.Error(".claude.json symlink should exist")
		}
	})

	t.Run("claude binary wrapper has cd workspace", func(t *testing.T) {
		out, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)"`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "cd /home/agent/workspace") {
			t.Errorf("claude wrapper should contain cd: got %s", out)
		}
	})

	t.Run("host paths rewritten in claude config", func(t *testing.T) {
		// Check that no JSON files under ~/.claude contain host home dir
		out, err := testDocker.SandboxExec(sb.name, "sh", "-c",
			"grep -rl '"+os.Getenv("HOME")+"' "+sandbox.SandboxClaudeDir+"/ 2>/dev/null || true")
		if err != nil {
			t.Fatalf("grep failed: %v", err)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("files still contain host path %s:\n%s", os.Getenv("HOME"), out)
		}
	})

	t.Run("sandbox run starts without cwd error", func(t *testing.T) {
		// Verifies the OCI runtime can chdir into the workspace mount.
		// This catches the deleted-temp-dir bug where docker sandbox run
		// fails with "no such file or directory" before any command runs.
		cmd := exec.Command("docker", "sandbox", "run", sb.name, "--", "--version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("sandbox run failed (cwd likely invalid): %s", out)
		}
	})
}
