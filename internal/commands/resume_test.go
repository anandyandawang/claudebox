// internal/commands/resume_test.go
package commands

import (
	"claudebox/internal/docker"
	"testing"
)

func TestResumeNoSandboxes(t *testing.T) {
	md := &mockDocker{lsOutput: []docker.SandboxInfo{}}
	cmd := NewResumeCmd(md, t.TempDir())
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("resume should fail with no sandboxes")
	}
}
