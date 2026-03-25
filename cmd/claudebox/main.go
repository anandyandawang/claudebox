package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func main() {
	templatesDir, err := findTemplatesDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	_ = templatesDir // used in later tasks

	rootCmd := &cobra.Command{
		Use:   "claudebox [template] [workspace] [-- agent_args...]",
		Short: "Run Claude Code in sandboxed Docker containers",
		Long: `claudebox creates isolated Docker sandbox environments for Claude Code
with per-template toolchains and network restrictions.

Each run creates a new sandbox with a local copy of the repo,
so multiple sessions can work on independent branches in parallel.`,
	}

	rootCmd.AddCommand(
		&cobra.Command{Use: "ls", Short: "List all sandboxes", RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		}},
		&cobra.Command{Use: "rm", Short: "Remove sandboxes", RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		}},
		&cobra.Command{Use: "resume", Short: "Resume an existing sandbox", RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		}},
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// findTemplatesDir resolves the templates directory relative to the binary,
// following symlinks (same behavior as the Bash version).
func findTemplatesDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("cannot resolve symlinks: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "templates"), nil
}
