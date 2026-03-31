//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceIsolation(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-isolation-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	t.Run("host workspace path not mounted", func(t *testing.T) {
		out, _ := testDocker.SandboxExec(name, "findmnt", "-t", "virtiofs", "-n", "-o", "TARGET")
		if strings.Contains(out, workspace) {
			t.Errorf("host workspace path should not appear in mounts: %s", out)
		}
	})

	t.Run("workspace copy has repo files", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "test", "-f", "/home/agent/workspace/testfile.txt")
		if err != nil {
			t.Error("testfile.txt should exist in workspace copy")
		}
	})

	t.Run("workspace copy is writable", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "touch", "/home/agent/workspace/new-file")
		if err != nil {
			t.Errorf("should be able to write to workspace copy: %v", err)
		}
	})
}

func TestDeadMountEscapeAttempts(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-deadmount-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	// Find the dead mount path (the temp dir that was deleted)
	out, err := testDocker.SandboxExec(name, "findmnt", "-t", "virtiofs", "-n", "-o", "TARGET")
	if err != nil {
		t.Fatalf("findmnt failed: %v", err)
	}
	// The dead mount is the first virtiofs target (primary workspace)
	mountPaths := strings.Fields(out)
	if len(mountPaths) == 0 {
		t.Fatal("no virtiofs mounts found")
	}
	deadMount := mountPaths[0]

	t.Run("write to dead mount fails", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "touch", deadMount+"/escape-test")
		if err == nil {
			t.Error("should not be able to write to dead mount")
		}
	})

	t.Run("mkdir in dead mount does not propagate to host", func(t *testing.T) {
		// mkdir may succeed in sandbox overlay
		testDocker.SandboxExec(name, "mkdir", "-p", deadMount+"/escape-dir")
		// But it must NOT appear on the host
		if _, err := os.Stat(deadMount); err == nil {
			// Path exists on host — check if the sandbox created it
			if _, err := os.Stat(filepath.Join(deadMount, "escape-dir")); err == nil {
				t.Error("sandbox mkdir propagated to host filesystem")
			}
		}
	})

	t.Run("write to re-created dir does not propagate to host", func(t *testing.T) {
		testDocker.SandboxExec(name, "mkdir", "-p", deadMount+"/write-test-dir")
		testDocker.SandboxExec(name, "touch", deadMount+"/write-test-dir/escape-file")
		if _, err := os.Stat(filepath.Join(deadMount, "write-test-dir", "escape-file")); err == nil {
			t.Error("file written to re-created dir propagated to host")
		}
	})
}

func TestHostDockerDaemonIsolation(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-docker-iso-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	t.Run("host docker socket not accessible", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name, "stat", "/var/run/docker.sock")
		if err == nil {
			out, _ := testDocker.SandboxExec(name, "docker", "info", "--format", "{{.Name}}")
			if !strings.Contains(out, "docker-desktop") && out != "" {
				t.Logf("warning: docker socket exists, docker info name=%q — verify this is the sandbox daemon", out)
			}
		}
	})

	t.Run("host docker daemon not reachable over TCP", func(t *testing.T) {
		_, err := testDocker.SandboxExec(name,
			"curl", "--connect-timeout", "3", "-sf", "http://host.docker.internal:2375/info")
		if err == nil {
			t.Error("should not be able to reach host Docker daemon over TCP")
		}
	})

	// VM boundary
	t.Run("host paths not reachable via inner docker", func(t *testing.T) {
		out, err := testDocker.SandboxExec(name,
			"sh", "-c", "docker run --rm -v /Users:/test alpine ls /test 2>&1 || true")
		if err == nil && strings.Contains(out, "andywang") {
			t.Error("inner docker should not see host /Users directory")
		}
	})

	t.Run("mount root is VM-scoped", func(t *testing.T) {
		sandboxHostname, _ := testDocker.SandboxExec(name, "hostname")
		innerHostname, err := testDocker.SandboxExec(name,
			"sh", "-c", "docker run --rm -v /:/mnt alpine cat /mnt/etc/hostname 2>&1 || true")
		if err == nil && innerHostname != "" && innerHostname != sandboxHostname {
			t.Logf("sandbox hostname=%q, inner mount hostname=%q — verify inner sees sandbox, not host", sandboxHostname, innerHostname)
		}
	})

	// Inner Docker escape attempts against dead mount
	t.Run("inner docker can't write to dead mount on host", func(t *testing.T) {
		out, _ := testDocker.SandboxExec(name, "findmnt", "-t", "virtiofs", "-n", "-o", "TARGET")
		deadMount := strings.Fields(out)[0]

		testDocker.SandboxExec(name,
			"sh", "-c", "docker run --rm -v "+deadMount+":/repo alpine touch /repo/docker-escape-test 2>&1 || true")
		if _, err := os.Stat(filepath.Join(deadMount, "docker-escape-test")); err == nil {
			t.Error("inner docker write to dead mount propagated to host")
		}
	})

	t.Run("inner docker can't write to arbitrary host paths", func(t *testing.T) {
		marker := filepath.Join(os.TempDir(), "claudebox-escape-marker")
		os.Remove(marker) // clean slate
		testDocker.SandboxExec(name,
			"sh", "-c", "docker run --rm -v /tmp:/t alpine touch /t/claudebox-escape-marker 2>&1 || true")
		if _, err := os.Stat(marker); err == nil {
			os.Remove(marker)
			t.Error("inner docker wrote to host /tmp")
		}
	})
}

func TestSandboxEscapeAttempt(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-escape-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	// Comprehensive escape attempt: write a script and try to get it to execute on the host
	marker := filepath.Join(os.TempDir(), "claudebox-escape-proof")
	os.Remove(marker)
	defer os.Remove(marker)

	// Attempt 1: Write to common auto-exec paths
	autoExecPaths := []string{
		"/tmp/claudebox-escape-proof",
		"/var/tmp/claudebox-escape-proof",
	}
	for _, p := range autoExecPaths {
		testDocker.SandboxExec(name, "sh", "-c",
			"echo '#!/bin/sh\ntouch "+marker+"' > "+p+" 2>/dev/null; chmod +x "+p+" 2>/dev/null || true")
	}

	// Attempt 2: Try to write via inner Docker to host temp
	testDocker.SandboxExec(name, "sh", "-c",
		"docker run --rm -v /tmp:/t alpine sh -c 'echo touched > /t/claudebox-escape-proof' 2>&1 || true")

	// Verify: marker file must not exist on host
	if _, err := os.Stat(marker); err == nil {
		t.Error("SANDBOX ESCAPE: marker file appeared on host filesystem")
	}
}
