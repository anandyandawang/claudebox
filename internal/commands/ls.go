// internal/commands/ls.go
package commands

import (
	"claudebox/internal/docker"
	"fmt"

	"github.com/spf13/cobra"
)

func NewLsCmd(d docker.Docker) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all sandboxes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sandboxes, err := d.SandboxLs("")
			if err != nil {
				return err
			}
			for _, s := range sandboxes {
				fmt.Fprintln(cmd.OutOrStdout(), s.Name)
			}
			return nil
		},
	}
}
