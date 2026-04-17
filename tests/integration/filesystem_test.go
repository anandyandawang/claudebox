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
	workspace := createTestWorkspaceWithBareOrigin(t, "cb-fs-test")
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

	t.Run("session branch matches origin/main tip", func(t *testing.T) {
		sessionTip, err := testDocker.SandboxExec(sb.name, "git", "-C", sandbox.SandboxWorkspace,
			"rev-parse", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		originTip, err := testDocker.SandboxExec(sb.name, "git", "-C", sandbox.SandboxWorkspace,
			"rev-parse", "origin/main")
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(sessionTip) != strings.TrimSpace(originTip) {
			t.Errorf("session HEAD %q != origin/main %q",
				strings.TrimSpace(sessionTip), strings.TrimSpace(originTip))
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

	t.Run("sandbox run starts without cwd error", func(t *testing.T) {
		// Verifies the OCI runtime can chdir into the workspace mount.
		// This catches the deleted-temp-dir bug where docker sandbox run
		// fails with "no such file or directory" before any command runs.
		// Runs before re-wrap tests that replace claude-real with fakes.
		cmd := exec.Command("docker", "sandbox", "run", sb.name, "--", "--version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("sandbox run failed (cwd likely invalid): %s", out)
		}
	})

	t.Run("re-wrap after binary replacement restores wrapper", func(t *testing.T) {
		// Simulate Claude Code auto-update: overwrite the wrapper with a fake binary.
		_, err := testDocker.SandboxExec(sb.name, "sh", "-c",
			`sudo tee "$(which claude)" > /dev/null <<'BIN'
#!/bin/bash
echo "I am a new claude binary"
BIN`)
		if err != nil {
			t.Fatal(err)
		}

		// Re-wrap (as resume would do). Sentinel is missing from the fake binary,
		// so it should be moved to claude-real (replacing the old one).
		if err := testManager.WrapClaudeBinary(sb.name); err != nil {
			t.Fatal(err)
		}

		// Wrapper should be restored.
		out, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)"`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "cd /home/agent/workspace") {
			t.Errorf("wrapper should be restored with cd, got: %s", out)
		}
		if !strings.Contains(out, "CLAUDEBOX_WRAPPER") {
			t.Errorf("wrapper should contain sentinel, got: %s", out)
		}
		if !strings.Contains(out, "claude-real") {
			t.Errorf("wrapper should exec claude-real, got: %s", out)
		}

		// claude-real should now be the NEW fake binary (sentinel detected the replacement).
		realAfter, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)-real"`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(realAfter, "new claude binary") {
			t.Errorf("claude-real should be the auto-updated binary, got: %s", realAfter)
		}
	})

	t.Run("re-wrap after full binary replacement uses mv path", func(t *testing.T) {
		// Simulate full package replacement: both claude (wrapper) and claude-real are gone.
		// Write a fake binary to claude and remove claude-real to trigger the mv guard.
		_, err := testDocker.SandboxExec(sb.name, "sh", "-c",
			`CLAUDE_BIN=$(which claude) && sudo rm -f "${CLAUDE_BIN}-real" && sudo tee "$CLAUDE_BIN" > /dev/null <<'BIN'
#!/bin/bash
echo "I am a fully replaced claude binary"
BIN
sudo chmod +x "$CLAUDE_BIN"`)
		if err != nil {
			t.Fatal(err)
		}

		// Verify claude-real is gone.
		_, err = testDocker.SandboxExec(sb.name, "sh", "-c", `test -f "$(which claude)-real"`)
		if err == nil {
			t.Fatal("claude-real should not exist before re-wrap")
		}

		// Re-wrap — should trigger the mv path (move fake binary to claude-real).
		if err := testManager.WrapClaudeBinary(sb.name); err != nil {
			t.Fatal(err)
		}

		// Wrapper should be in place.
		out, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)"`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "cd /home/agent/workspace") {
			t.Errorf("wrapper should contain cd, got: %s", out)
		}

		// claude-real should now exist (mv path created it from the fake binary).
		realOut, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)-real"`)
		if err != nil {
			t.Fatal("claude-real should exist after re-wrap via mv path")
		}
		if !strings.Contains(realOut, "fully replaced claude binary") {
			t.Errorf("claude-real should be the fake binary that was moved, got: %s", realOut)
		}
	})

	t.Run("re-wrap is idempotent when wrapper intact", func(t *testing.T) {
		// Ensure wrapper is in place from a prior test.
		if err := testManager.WrapClaudeBinary(sb.name); err != nil {
			t.Fatal(err)
		}

		// Capture claude-real before a second re-wrap.
		realBefore, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)-real"`)
		if err != nil {
			t.Fatal(err)
		}

		// Re-wrap again — wrapper has the sentinel, so claude-real should NOT change.
		if err := testManager.WrapClaudeBinary(sb.name); err != nil {
			t.Fatal(err)
		}

		realAfter, err := testDocker.SandboxExec(sb.name, "sh", "-c", `cat "$(which claude)-real"`)
		if err != nil {
			t.Fatal(err)
		}
		if realAfter != realBefore {
			t.Errorf("claude-real should be unchanged when wrapper is intact:\nbefore: %s\nafter: %s", realBefore, realAfter)
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

}
