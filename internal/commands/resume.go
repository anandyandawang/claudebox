// internal/commands/resume.go
package commands

import (
	"bufio"
	"claudebox/internal/credentials"
	"claudebox/internal/docker"
	"claudebox/internal/sandbox"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func NewResumeCmd(d docker.Docker, templatesDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "resume [-- agent_args...]",
		Short: "Resume an existing sandbox",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResume(d, templatesDir, args, os.Stdin)
		},
	}
}

func runResume(d docker.Docker, templatesDir string, agentArgs []string, stdin *os.File) error {
	mgr := sandbox.NewManager(d, templatesDir)
	reader := bufio.NewReader(stdin)

	// List sandboxes for current workspace
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	prefix := sandbox.WorkspacePrefix(wd)
	names, err := mgr.List(prefix)
	if err != nil {
		return err
	}

	if len(names) == 0 {
		return fmt.Errorf("no sandboxes found for this workspace")
	}

	var sandboxName string

	if len(names) == 1 {
		fmt.Printf("Resume %s? [Y/n]: ", names[0])
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "n") {
			return nil
		}
		sandboxName = names[0]
	} else {
		fmt.Println("Available sandboxes:")
		for i, name := range names {
			fmt.Printf("  %d) %s\n", i+1, name)
		}
		for {
			fmt.Printf("Pick a sandbox [1-%d]: ", len(names))
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			pick, err := strconv.Atoi(line)
			if err == nil && pick >= 1 && pick <= len(names) {
				sandboxName = names[pick-1]
				break
			}
			fmt.Printf("Invalid selection. Enter a number between 1 and %d.\n", len(names))
		}
	}

	fmt.Printf("Resuming sandbox: %s...\n", sandboxName)

	if err := credentials.Refresh(d, sandboxName); err != nil {
		return err
	}
	if err := mgr.WrapClaudeBinary(sandboxName); err != nil {
		return err
	}

	fmt.Println("Starting sandbox...")
	runArgs := append([]string{"--dangerously-skip-permissions"}, agentArgs...)
	return mgr.Run(sandboxName, runArgs...)
}
