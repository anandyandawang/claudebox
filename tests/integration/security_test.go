//go:build integration

package integration

import (
	"claudebox/internal/sandbox"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// findDeadMount returns the virtiofs mount path containing "claudebox-" (the deleted temp dir).
func findDeadMount(t *testing.T, name string) string {
	t.Helper()
	out, err := testDocker.SandboxExec(name, "findmnt", "-t", "virtiofs", "-n", "-o", "TARGET")
	if err != nil {
		t.Fatalf("findmnt failed: %v", err)
	}
	for _, p := range strings.Fields(out) {
		if strings.Contains(p, "claudebox-") {
			return p
		}
	}
	return ""
}

func TestSecuritySuite(t *testing.T) {
	workspace := createTestWorkspace(t, "cb-security-test")
	buildTemplateImage(t, "jvm")
	name := createTestSandbox(t, "jvm", workspace)
	defer cleanupSandbox(t, name)

	t.Run("WorkspaceIsolation", func(t *testing.T) {
		t.Run("host workspace path not mounted", func(t *testing.T) {
			out, _ := testDocker.SandboxExec(name, "findmnt", "-t", "virtiofs", "-n", "-o", "TARGET")
			if strings.Contains(out, workspace) {
				t.Errorf("host workspace path should not appear in mounts: %s", out)
			}
		})

		t.Run("workspace copy has repo files", func(t *testing.T) {
			_, err := testDocker.SandboxExec(name, "test", "-f", sandbox.SandboxWorkspace+"/testfile.txt")
			if err != nil {
				t.Error("testfile.txt should exist in workspace copy")
			}
		})

		t.Run("workspace copy is writable", func(t *testing.T) {
			_, err := testDocker.SandboxExec(name, "touch", sandbox.SandboxWorkspace+"/new-file")
			if err != nil {
				t.Errorf("should be able to write to workspace copy: %v", err)
			}
		})
	})

	t.Run("MountIsolation", func(t *testing.T) {
		emptyMount := findDeadMount(t, name)
		if emptyMount == "" {
			t.Fatal("no virtiofs mount with claudebox- prefix found")
		}

		t.Run("host mount dir is empty", func(t *testing.T) {
			entries, err := os.ReadDir(emptyMount)
			if err != nil {
				t.Fatalf("reading mount dir: %v", err)
			}
			if len(entries) != 0 {
				t.Errorf("mount dir should be empty on host, got %d entries", len(entries))
			}
		})

		t.Run("sandbox writes do not propagate to host", func(t *testing.T) {
			testDocker.SandboxExec(name, "mkdir", "-p", emptyMount+"/escape-dir")
			testDocker.SandboxExec(name, "touch", emptyMount+"/escape-dir/escape-file")
			if _, err := os.Stat(filepath.Join(emptyMount, "escape-dir")); err == nil {
				t.Error("sandbox mkdir propagated to host filesystem")
			}
		})

		t.Run("sandbox file writes do not appear on host", func(t *testing.T) {
			testDocker.SandboxExec(name, "sh", "-c", "echo secret > "+emptyMount+"/leak.txt 2>/dev/null || true")
			if _, err := os.Stat(filepath.Join(emptyMount, "leak.txt")); err == nil {
				t.Error("sandbox file write propagated to host")
			}
		})
	})

	t.Run("HostDockerDaemonIsolation", func(t *testing.T) {
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

		t.Run("host paths not reachable via inner docker", func(t *testing.T) {
			u, _ := user.Current()
			out, err := testDocker.SandboxExec(name,
				"sh", "-c", "docker run --rm -v /Users:/test alpine ls /test 2>&1 || true")
			if err == nil && u != nil && strings.Contains(out, u.Username) {
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

		t.Run("inner docker can't write to dead mount on host", func(t *testing.T) {
			deadMount := findDeadMount(t, name)
			if deadMount == "" {
				t.Skip("no claudebox- virtiofs mount found")
			}
			testDocker.SandboxExec(name,
				"sh", "-c", "docker run --rm -v "+deadMount+":/repo alpine touch /repo/docker-escape-test 2>&1 || true")
			if _, err := os.Stat(filepath.Join(deadMount, "docker-escape-test")); err == nil {
				t.Error("inner docker write to dead mount propagated to host")
			}
		})

		t.Run("inner docker can't write to arbitrary host paths", func(t *testing.T) {
			marker := filepath.Join(os.TempDir(), "claudebox-escape-marker")
			os.Remove(marker)
			testDocker.SandboxExec(name,
				"sh", "-c", "docker run --rm -v /tmp:/t alpine touch /t/claudebox-escape-marker 2>&1 || true")
			if _, err := os.Stat(marker); err == nil {
				os.Remove(marker)
				t.Error("inner docker wrote to host /tmp")
			}
		})
	})

	t.Run("SandboxEscapeAttempt", func(t *testing.T) {
		marker := filepath.Join(os.TempDir(), "claudebox-escape-proof")
		os.Remove(marker)
		defer os.Remove(marker)

		autoExecPaths := []string{
			"/tmp/claudebox-escape-proof",
			"/var/tmp/claudebox-escape-proof",
		}
		for _, p := range autoExecPaths {
			testDocker.SandboxExec(name, "sh", "-c",
				"echo '#!/bin/sh\ntouch "+marker+"' > "+p+" 2>/dev/null; chmod +x "+p+" 2>/dev/null || true")
		}

		testDocker.SandboxExec(name, "sh", "-c",
			"docker run --rm -v /tmp:/t alpine sh -c 'echo touched > /t/claudebox-escape-proof' 2>&1 || true")

		if _, err := os.Stat(marker); err == nil {
			t.Error("SANDBOX ESCAPE: marker file appeared on host filesystem")
		}
	})
}
