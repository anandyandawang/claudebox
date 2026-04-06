// cmd/claudebox/main.go
package main

import (
	"claudebox/internal/cache"
	"claudebox/internal/commands"
	"claudebox/internal/docker"
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

	d := docker.NewClient()

	// Prune stale image-cache tars before running any command.
	imageCacheDir := filepath.Join(os.Getenv("HOME"), ".docker", "sandboxes", "image-cache")
	cache.PruneImageCache(imageCacheDir)

	rootCmd := &cobra.Command{
		Use:   "claudebox [template] [workspace] [-- agent_args...]",
		Short: "Run Claude Code in sandboxed Docker containers",
		Long: `claudebox creates isolated Docker sandbox environments for Claude Code
with per-template toolchains and network restrictions.

Each run creates a new sandbox with a local copy of the repo,
so multiple sessions can work on independent branches in parallel.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return commands.RunCreate(d, templatesDir, args)
		},
	}

	rootCmd.AddCommand(
		commands.NewLsCmd(d),
		commands.NewRmCmd(d),
		commands.NewResumeCmd(d, templatesDir),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

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
