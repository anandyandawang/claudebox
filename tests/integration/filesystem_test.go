//go:build integration

package integration

import (
	"claudebox/internal/sandbox"
	"fmt"
	"net/url"
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

	t.Run("host git identity imported into sandbox", func(t *testing.T) {
		hostNameOut, _ := exec.Command("git", "config", "--global", "user.name").Output()
		hostEmailOut, _ := exec.Command("git", "config", "--global", "user.email").Output()
		hostName := strings.TrimSpace(string(hostNameOut))
		hostEmail := strings.TrimSpace(string(hostEmailOut))
		if hostName == "" && hostEmail == "" {
			t.Skip("host has no global git identity")
		}

		if hostName != "" {
			got, err := testDocker.SandboxExec(sb.name, "git", "config", "--global", "user.name")
			if err != nil {
				t.Fatalf("reading sandbox user.name: %v", err)
			}
			if strings.TrimSpace(got) != hostName {
				t.Errorf("sandbox user.name = %q, want %q", strings.TrimSpace(got), hostName)
			}
		}
		if hostEmail != "" {
			got, err := testDocker.SandboxExec(sb.name, "git", "config", "--global", "user.email")
			if err != nil {
				t.Fatalf("reading sandbox user.email: %v", err)
			}
			if strings.TrimSpace(got) != hostEmail {
				t.Errorf("sandbox user.email = %q, want %q", strings.TrimSpace(got), hostEmail)
			}
		}
	})

	t.Run("GITHUB_USERNAME exported in sandbox-persistent.sh", func(t *testing.T) {
		hostValue := os.Getenv("GITHUB_USERNAME")
		if hostValue == "" {
			t.Skip("GITHUB_USERNAME not set on host")
		}

		got, err := testDocker.SandboxExec(sb.name, "sh", "-c",
			`. /etc/sandbox-persistent.sh && printf %s "$GITHUB_USERNAME"`)
		if err != nil {
			t.Fatalf("sourcing sandbox-persistent.sh: %v", err)
		}
		if strings.TrimSpace(got) != hostValue {
			t.Errorf("sandbox GITHUB_USERNAME = %q, want %q", strings.TrimSpace(got), hostValue)
		}
	})

	t.Run("JAVA_TOOL_OPTIONS written when HTTPS_PROXY is set on host", func(t *testing.T) {
		proxy := os.Getenv("HTTPS_PROXY")
		if proxy == "" {
			t.Skip("HTTPS_PROXY not set on host")
		}

		u, err := url.Parse(proxy)
		if err != nil || u.Hostname() == "" || u.Port() == "" {
			t.Fatalf("cannot parse HTTPS_PROXY=%q: %v", proxy, err)
		}
		host, port := u.Hostname(), u.Port()

		got, err := testDocker.SandboxExec(sb.name, "sh", "-c",
			`. /etc/sandbox-persistent.sh && printf %s "$JAVA_TOOL_OPTIONS"`)
		if err != nil {
			t.Fatalf("sourcing sandbox-persistent.sh: %v", err)
		}
		opts := strings.TrimSpace(got)

		want := []string{
			fmt.Sprintf("-Dhttp.proxyHost=%s", host),
			fmt.Sprintf("-Dhttp.proxyPort=%s", port),
			fmt.Sprintf("-Dhttps.proxyHost=%s", host),
			fmt.Sprintf("-Dhttps.proxyPort=%s", port),
		}
		for _, w := range want {
			if !strings.Contains(opts, w) {
				t.Errorf("JAVA_TOOL_OPTIONS missing %q; got: %q", w, opts)
			}
		}
	})

}

func TestCreateAbortsWithoutOrigin(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-no-origin-test") // no remote configured
	buildTemplateImage(t, "jvm")

	sandboxID := sandbox.GenerateSandboxID("jvm")
	name := sandbox.GenerateSandboxName(workspace, sandboxID)
	defer cleanupSandbox(t, name)

	err := testManager.Create(name, sandbox.CreateOpts{
		ImageName: "jvm-sandbox",
		Workspace: workspace,
		ClaudeDir: os.Getenv("HOME") + "/.claude",
		SessionID: sandboxID,
	})
	if err == nil {
		t.Fatal("expected Create to abort when workspace has no origin remote")
	}
	if !strings.Contains(err.Error(), "determining default branch") {
		t.Errorf("error should mention default branch discovery, got: %v", err)
	}
}
