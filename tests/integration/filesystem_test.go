//go:build integration

package integration

import (
	"strings"
	"testing"
)

func TestFilesystemLayout(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-fs-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	t.Run("workspace files exist", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "test", "-f", "/home/agent/workspace/testfile.txt")
		if err != nil {
			t.Error("testfile.txt should exist in workspace")
		}
	})

	t.Run("git branch matches sandbox pattern", func(t *testing.T) {
		branch, err := testDocker.SandboxExec(name, "git", "-C", "/home/agent/workspace", "branch", "--show-current")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(branch, "sandbox-") {
			t.Errorf("branch %q should start with sandbox-", branch)
		}
	})

	t.Run("claude config symlinks exist", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "test", "-L", "/home/agent/.claude.json")
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
}
