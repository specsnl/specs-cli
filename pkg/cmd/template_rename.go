package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	"github.com/specsnl/specs-cli/pkg/util/output"
	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func newTemplateRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rename <old-name> <new-name>",
		Aliases: []string{"mv"},
		Short:   "Rename a registered template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]

			if err := validate.Name(newName); err != nil {
				return err
			}
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			src := specs.TemplatePath(oldName)
			if _, err := os.Stat(src); os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, oldName)
			}

			dst := specs.TemplatePath(newName)
			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf("name %q already exists — delete it first", newName)
			}

			if err := os.Rename(src, dst); err != nil {
				return err
			}

			output.Info("template %q renamed to %q", oldName, newName)
			return nil
		},
	}
}
