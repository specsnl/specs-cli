package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func newTemplateDeleteCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name> [<name>...]",
		Aliases: []string{"remove", "rm", "del"},
		Short:   "Delete one or more registered templates",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			for _, name := range args {
				if err := validate.Name(name); err != nil {
					return err
				}
				path := specs.TemplatePath(name)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					return fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, name)
				}
				if err := os.RemoveAll(path); err != nil {
					return err
				}
				app.Output.Info("template %q deleted", name)
			}
			return nil
		},
	}
}
