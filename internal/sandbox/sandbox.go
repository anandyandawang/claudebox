// internal/sandbox/sandbox.go
package sandbox

import (
	"bufio"
	"claudebox/internal/docker"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles sandbox lifecycle operations.
type Manager struct {
	docker       docker.Docker
	templatesDir string
}

// CreateOpts holds options for creating a sandbox.
type CreateOpts struct {
	ImageName string
	Workspace string
	ClaudeDir string
	SessionID string
}

// NewManager returns a Manager.
func NewManager(d docker.Docker, templatesDir string) *Manager {
	return &Manager{docker: d, templatesDir: templatesDir}
}

// ValidateTemplate checks the template directory contains a Dockerfile.
func (m *Manager) ValidateTemplate(template string) error {
	df := filepath.Join(m.templatesDir, template, "Dockerfile")
	if _, err := os.Stat(df); err != nil {
		return fmt.Errorf("no Dockerfile found in %s", filepath.Join(m.templatesDir, template))
	}
	return nil
}

// BuildImage builds the Docker image for a template. Returns image name.
func (m *Manager) BuildImage(template string) (string, error) {
	imageName := template + "-sandbox"
	if err := m.docker.Build(imageName, filepath.Join(m.templatesDir, template)); err != nil {
		return "", fmt.Errorf("building image: %w", err)
	}
	return imageName, nil
}

// Create creates a sandbox with tar-piped workspace and config, no host mounts.
func (m *Manager) Create(sandboxName string, opts CreateOpts) error {
	// Create and immediately delete a temp dir for the required workspace arg.
	// After deletion, the virtiofs mount inside the sandbox becomes a dead end.
	tmpDir, err := os.MkdirTemp("", "claudebox-")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	if err := m.docker.SandboxCreate(sandboxName, docker.SandboxCreateOpts{
		Image:     opts.ImageName,
		Command:   "claude",
		Workspace: tmpDir,
	}); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("creating sandbox: %w", err)
	}
	os.RemoveAll(tmpDir)

	if err := m.tarPipeTo(sandboxName, opts.Workspace, "/home/agent/workspace/"); err != nil {
		return fmt.Errorf("copying workspace: %w", err)
	}
	if err := m.tarPipeClaudeConfig(sandboxName, opts.ClaudeDir); err != nil {
		return fmt.Errorf("copying claude config: %w", err)
	}
	if _, err := m.docker.SandboxExec(sandboxName, "git", "-C", "/home/agent/workspace", "clean", "-fdx", "-q"); err != nil {
		return fmt.Errorf("cleaning workspace: %w", err)
	}
	if _, err := m.docker.SandboxExec(sandboxName, "git", "-C", "/home/agent/workspace", "checkout", "-b", opts.SessionID); err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}
	return nil
}

// tarPipeTo tars srcDir on the host and extracts into destDir in the sandbox.
// If paths are provided, only those entries are tarred; otherwise the entire directory.
func (m *Manager) tarPipeTo(sandboxName, srcDir, destDir string, paths ...string) error {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	tarArgs := append([]string{"-C", srcDir, "-c"}, paths...)
	tarCmd := exec.Command("tar", tarArgs...)
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	tarCmd.Stdout = pw
	if err := tarCmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return err
	}
	pw.Close()
	extractErr := m.docker.SandboxExecWithStdin(pr, sandboxName, "tar", "-C", destDir, "-x")
	pr.Close()
	if waitErr := tarCmd.Wait(); waitErr != nil {
		if extractErr != nil {
			return fmt.Errorf("tar create: %w; extract: %v", waitErr, extractErr)
		}
		return fmt.Errorf("tar create: %w", waitErr)
	}
	return extractErr
}

// collectConfigFiles returns the subset of candidates that exist under dir.
// Directories are only included if they are actually directories.
func collectConfigFiles(dir string, candidates []string) []string {
	var files []string
	for _, f := range candidates {
		info, err := os.Stat(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		if info.IsDir() || !info.IsDir() {
			files = append(files, f)
		}
	}
	return files
}

// tarPipeClaudeConfig copies .claude.json, settings.json, and plugins/ into the sandbox.
func (m *Manager) tarPipeClaudeConfig(sandboxName, claudeDir string) error {
	files := collectConfigFiles(claudeDir, []string{".claude.json", "settings.json", "plugins"})
	if len(files) == 0 {
		return nil
	}

	if _, err := m.docker.SandboxExec(sandboxName, "mkdir", "-p", "/home/agent/.claude"); err != nil {
		return fmt.Errorf("creating .claude dir: %w", err)
	}
	if err := m.tarPipeTo(sandboxName, claudeDir, "/home/agent/.claude", files...); err != nil {
		return err
	}

	// Symlink .claude.json to home dir only if the file was actually copied
	for _, f := range files {
		if f == ".claude.json" {
			if _, err := m.docker.SandboxExec(sandboxName, "ln", "-sf", "/home/agent/.claude/.claude.json", "/home/agent/.claude.json"); err != nil {
				return fmt.Errorf("symlinking .claude.json: %w", err)
			}
			break
		}
	}
	return nil
}

// RefreshConfig re-copies settings.json and plugins/ from the host into the sandbox.
// Called on resume to pick up any host-side changes.
func (m *Manager) RefreshConfig(sandboxName, claudeDir string) error {
	files := collectConfigFiles(claudeDir, []string{"settings.json", "plugins"})
	if len(files) == 0 {
		return nil
	}

	if _, err := m.docker.SandboxExec(sandboxName, "mkdir", "-p", "/home/agent/.claude"); err != nil {
		return fmt.Errorf("creating .claude dir: %w", err)
	}
	return m.tarPipeTo(sandboxName, claudeDir, "/home/agent/.claude", files...)
}

// ApplyNetworkPolicy reads allowed-hosts.txt and applies deny-by-default network policy.
// Returns true if a policy was applied.
func (m *Manager) ApplyNetworkPolicy(sandboxName, template string) (bool, error) {
	hostsFile := filepath.Join(m.templatesDir, template, "allowed-hosts.txt")
	f, err := os.Open(hostsFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()

	var hosts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		hosts = append(hosts, line)
	}
	if err := m.docker.SandboxNetworkProxy(sandboxName, hosts); err != nil {
		return false, fmt.Errorf("applying network policy: %w", err)
	}
	return true, nil
}

// VerifyNetworkPolicy checks that the firewall blocks example.com and allows api.github.com.
// Both checks are always performed.
func (m *Manager) VerifyNetworkPolicy(sandboxName string) error {
	_, blockedErr := m.docker.SandboxExec(sandboxName,
		"curl", "--connect-timeout", "5", "-sf", "https://example.com")
	_, allowedErr := m.docker.SandboxExec(sandboxName,
		"curl", "--connect-timeout", "5", "-sf", "https://api.github.com/zen")
	if blockedErr == nil {
		return fmt.Errorf("firewall verification failed - was able to reach https://example.com")
	}
	if allowedErr != nil {
		return fmt.Errorf("firewall verification failed - unable to reach https://api.github.com")
	}
	return nil
}

// WrapClaudeBinary wraps the claude binary to cd to /home/agent/workspace first.
func (m *Manager) WrapClaudeBinary(sandboxName string) error {
	script := `CLAUDE_BIN=$(which claude)
if [ ! -f "${CLAUDE_BIN}-real" ]; then
  sudo mv "$CLAUDE_BIN" "${CLAUDE_BIN}-real"
fi
sudo tee "$CLAUDE_BIN" > /dev/null << 'WRAPPER'
#!/bin/bash
cd /home/agent/workspace
exec "$(dirname "$0")/claude-real" "$@"
WRAPPER
sudo chmod +x "$CLAUDE_BIN"`
	if _, err := m.docker.SandboxExec(sandboxName, "sh", "-c", script); err != nil {
		return fmt.Errorf("wrapping claude binary: %w", err)
	}
	return nil
}

// Run starts a sandbox.
func (m *Manager) Run(sandboxName string, args ...string) error {
	return m.docker.SandboxRun(sandboxName, args...)
}

// List returns sandbox names matching the prefix.
func (m *Manager) List(workspacePrefix string) ([]string, error) {
	sandboxes, err := m.docker.SandboxLs(workspacePrefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, s := range sandboxes {
		names = append(names, s.Name)
	}
	return names, nil
}

// Remove deletes a single sandbox.
func (m *Manager) Remove(name string) error {
	return m.docker.SandboxRm(name)
}

// RemoveAll deletes all sandboxes matching the prefix. Returns count removed.
func (m *Manager) RemoveAll(workspacePrefix string) (int, error) {
	sandboxes, err := m.docker.SandboxLs(workspacePrefix)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, s := range sandboxes {
		if err := m.docker.SandboxRm(s.Name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", s.Name, err)
			continue
		}
		count++
	}
	return count, nil
}
