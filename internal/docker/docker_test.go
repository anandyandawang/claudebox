package docker

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// captureCmd records the command args and returns a no-op command.
func captureCmd(calls *[][]string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return exec.Command("true")
	}
}

func TestBuild(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	if err := c.Build("jvm-sandbox", "/path/to/templates/jvm"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "build", "-t", "jvm-sandbox", "/path/to/templates/jvm"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Errorf("Build args:\n  got  %v\n  want %v", calls, want)
	}
}

func TestSandboxCreate(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	err := c.SandboxCreate("my-sandbox", SandboxCreateOpts{
		Image:     "jvm-sandbox",
		Command:   "claude",
		Workspace: "/tmp/claudebox-abc123",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "create", "-t", "jvm-sandbox",
		"--name", "my-sandbox", "claude", "/tmp/claudebox-abc123"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxCreate args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxExec(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		return exec.Command("echo", "output")
	}}

	out, err := c.SandboxExec("my-sandbox", "sh", "-c", "echo hello")
	if err != nil {
		t.Fatal(err)
	}
	if out != "output" {
		t.Errorf("output: got %q, want %q", out, "output")
	}
	want := []string{"docker", "sandbox", "exec", "my-sandbox", "sh", "-c", "echo hello"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxExec args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxLs(t *testing.T) {
	c := &Client{newCmd: func(name string, args ...string) *exec.Cmd {
		return exec.Command("printf", "NAME\tSTATUS\nfoo-sandbox\trunning\nbar-sandbox\tstopped\n")
	}}

	sandboxes, err := c.SandboxLs("")
	if err != nil {
		t.Fatal(err)
	}
	if len(sandboxes) != 2 {
		t.Fatalf("count: got %d, want 2", len(sandboxes))
	}
	if sandboxes[0].Name != "foo-sandbox" || sandboxes[1].Name != "bar-sandbox" {
		t.Errorf("names: got %v", sandboxes)
	}
}

func TestSandboxLsWithFilter(t *testing.T) {
	c := &Client{newCmd: func(name string, args ...string) *exec.Cmd {
		return exec.Command("printf", "NAME\tSTATUS\nfoo-sandbox\trunning\nbar-sandbox\tstopped\n")
	}}

	sandboxes, err := c.SandboxLs("foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(sandboxes) != 1 || sandboxes[0].Name != "foo-sandbox" {
		t.Errorf("filtered: got %v, want [foo-sandbox]", sandboxes)
	}
}

func TestSandboxRm(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	if err := c.SandboxRm("my-sandbox"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "rm", "my-sandbox"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxRm args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxRun(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	if err := c.SandboxRun("my-sandbox", "--dangerously-skip-permissions"); err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "run", "my-sandbox", "--", "--dangerously-skip-permissions"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxRun args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxExecWithStdin(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		// cat will read stdin and output it; we use it to verify stdin is wired
		return exec.Command("cat")
	}}

	input := strings.NewReader("hello from stdin")
	err := c.SandboxExecWithStdin(input, "my-sandbox", "sh", "-c", "tar -x")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "exec", "-i", "my-sandbox", "sh", "-c", "tar -x"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxExecWithStdin args:\n  got  %v\n  want %v", calls[0], want)
	}
}

func TestSandboxNetworkProxy(t *testing.T) {
	var calls [][]string
	c := &Client{newCmd: captureCmd(&calls)}

	err := c.SandboxNetworkProxy("my-sandbox", []string{"api.github.com", "registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "sandbox", "network", "proxy", "my-sandbox",
		"--policy", "deny", "--allow-host", "api.github.com", "--allow-host", "registry.npmjs.org"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("SandboxNetworkProxy args:\n  got  %v\n  want %v", calls[0], want)
	}
}
