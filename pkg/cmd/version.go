package cmd

import (
	"github.com/spf13/cobra"
)

// Version is the current binary version.
// Set at build time via: -ldflags "-X github.com/specsnl/specs-cli/pkg/cmd.Version=1.0.0"
var Version = "dev"

func newVersionCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the specs version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app.Output.Info("specs version %s", Version)
			return nil
		},
	}
}
