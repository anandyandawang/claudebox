// internal/commands/rm.go
package commands

import (
	"claudebox/internal/docker"
	"claudebox/internal/sandbox"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewRmCmd(d docker.Docker) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name|all>",
		Short: "Remove a sandbox or all sandboxes for the current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := sandbox.NewManager(d, "")

			if args[0] == "all" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				prefix := sandbox.WorkspacePrefix(wd)
				count, err := mgr.RemoveAll(prefix)
				if err != nil {
					return err
				}
				if count == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No sandboxes found for %s.\n", wd)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Removed %d sandbox(es).\n", count)
				}
				return nil
			}

			// Remove by name
			name := args[0]
			all, err := d.SandboxLs("")
			if err != nil {
				return err
			}
			found := false
			for _, s := range all {
				if s.Name == name {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("sandbox %s not found", name)
			}
			if err := mgr.Remove(name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed sandbox: %s\n", name)
			return nil
		},
	}
}
