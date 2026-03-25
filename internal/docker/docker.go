package docker

import (
	"os"
	"os/exec"
	"strings"
)

// Docker defines the interface for docker sandbox operations.
type Docker interface {
	Build(tag string, contextDir string) error
	SandboxCreate(name string, opts SandboxCreateOpts) error
	SandboxRun(name string, args ...string) error
	SandboxExec(name string, args ...string) (string, error)
	SandboxLs(filter string) ([]SandboxInfo, error)
	SandboxRm(name string) error
	SandboxNetworkProxy(name string, allowedHosts []string) error
}

// SandboxCreateOpts holds options for creating a sandbox.
type SandboxCreateOpts struct {
	Image   string   // Docker image tag
	Command string   // Base command (e.g. "claude")
	Mounts  []string // Positional args: workspace path, claude config dir
}

// SandboxInfo represents a sandbox from docker sandbox ls.
type SandboxInfo struct {
	Name string
}

// Client implements Docker by shelling out to the docker CLI.
type Client struct {
	newCmd func(name string, args ...string) *exec.Cmd
}

// NewClient returns a Client that runs real docker commands.
func NewClient() *Client {
	return &Client{newCmd: exec.Command}
}

func (c *Client) Build(tag string, contextDir string) error {
	cmd := c.newCmd("docker", "build", "-t", tag, contextDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Client) SandboxCreate(name string, opts SandboxCreateOpts) error {
	args := []string{"sandbox", "create", "-t", opts.Image, "--name", name, opts.Command}
	args = append(args, opts.Mounts...)
	cmd := c.newCmd("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Client) SandboxRun(name string, args ...string) error {
	cmdArgs := []string{"sandbox", "run", name}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, "--")
		cmdArgs = append(cmdArgs, args...)
	}
	cmd := c.newCmd("docker", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Client) SandboxExec(name string, args ...string) (string, error) {
	cmdArgs := append([]string{"sandbox", "exec", name}, args...)
	cmd := c.newCmd("docker", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) SandboxLs(filter string) ([]SandboxInfo, error) {
	cmd := c.newCmd("docker", "sandbox", "ls")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var sandboxes []SandboxInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		if i == 0 {
			continue // skip header
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if filter != "" && !strings.HasPrefix(name, filter) {
			continue
		}
		sandboxes = append(sandboxes, SandboxInfo{Name: name})
	}
	return sandboxes, nil
}

func (c *Client) SandboxRm(name string) error {
	cmd := c.newCmd("docker", "sandbox", "rm", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c *Client) SandboxNetworkProxy(name string, allowedHosts []string) error {
	args := []string{"sandbox", "network", "proxy", name, "--policy", "deny"}
	for _, host := range allowedHosts {
		args = append(args, "--allow-host", host)
	}
	cmd := c.newCmd("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
