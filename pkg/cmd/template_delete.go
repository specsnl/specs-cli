package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	"github.com/specsnl/specs-cli/pkg/util/output"
	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func newTemplateDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <tag> [<tag>...]",
		Short: "Delete one or more registered templates",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			for _, tag := range args {
				if err := validate.Tag(tag); err != nil {
					return err
				}
				path := specs.TemplatePath(tag)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					return fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, tag)
				}
				if err := os.RemoveAll(path); err != nil {
					return err
				}
				output.Info("template %q deleted", tag)
			}
			return nil
		},
	}
}
