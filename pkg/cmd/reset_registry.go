package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
)

func newResetRegistryCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:    "reset-registry",
		Short:  "Wipe and recreate the local template registry",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := specs.TemplateDir()
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			app.Output.Info("registry reset at %s", dir)
			return nil
		},
	}
}
